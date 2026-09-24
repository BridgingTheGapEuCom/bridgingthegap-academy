// Package postgres persists Translations-owned derived trees. It uses no
// Courses writes; source versions are read by the application service.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ translations.Repository = (*Repository)(nil)

func (r *Repository) Create(ctx context.Context, input translations.CreateInput) (translations.CourseTranslation, error) {
	if r == nil || r.pool == nil || input.Source.Validate() != nil || input.TargetLanguage == "" || input.TargetLanguage == input.Source.Language || !validUUID(input.CreatorUserID) || input.CreatedAt.IsZero() || input.Tree.ValidateForPersistence() != nil {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	tree, err := json.Marshal(input.Tree)
	if err != nil {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	result, err := scanTranslation(r.pool.QueryRow(ctx, `INSERT INTO translations.course_translation(source_course_id,source_course_version_id,source_version,source_language,target_language,creator_user_id,translated_tree,created_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)
RETURNING id,source_course_id,source_course_version_id,source_version,source_language,target_language,creator_user_id,status,revision,translated_tree,created_at,updated_at`, input.Source.CourseID, input.Source.CourseVersionID, input.Source.Version.String(), input.Source.Language, input.TargetLanguage, input.CreatorUserID, tree, input.CreatedAt.UTC()))
	if err != nil {
		return translations.CourseTranslation{}, storage(err)
	}
	return result, nil
}
func (r *Repository) Get(ctx context.Context, id translations.TranslationID) (translations.CourseTranslation, error) {
	if r == nil || r.pool == nil || !validUUID(string(id)) {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	result, err := scanTranslation(r.pool.QueryRow(ctx, `SELECT id,source_course_id,source_course_version_id,source_version,source_language,target_language,creator_user_id,status,revision,translated_tree,created_at,updated_at FROM translations.course_translation WHERE id=$1`, id))
	if err != nil {
		return translations.CourseTranslation{}, storage(err)
	}
	return result, nil
}
func (r *Repository) GetBySourceVersionAndLanguage(ctx context.Context, id courses.CourseVersionID, language courses.LanguageTag) (translations.CourseTranslation, error) {
	if r == nil || r.pool == nil || !validUUID(string(id)) || language == "" {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	result, err := scanTranslation(r.pool.QueryRow(ctx, `SELECT id,source_course_id,source_course_version_id,source_version,source_language,target_language,creator_user_id,status,revision,translated_tree,created_at,updated_at FROM translations.course_translation WHERE source_course_version_id=$1 AND target_language=$2`, id, language))
	if err != nil {
		return translations.CourseTranslation{}, storage(err)
	}
	return result, nil
}
func (r *Repository) UpdateTree(ctx context.Context, id translations.TranslationID, expected int64, tree translations.TranslationTree, at time.Time) (translations.CourseTranslation, error) {
	if r == nil || r.pool == nil || !validUUID(string(id)) || expected < 1 || at.IsZero() || tree.ValidateForPersistence() != nil {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	encoded, err := json.Marshal(tree)
	if err != nil {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	result, err := scanTranslation(r.pool.QueryRow(ctx, `UPDATE translations.course_translation SET translated_tree=$3,revision=revision+1,updated_at=$4 WHERE id=$1 AND revision=$2 RETURNING id,source_course_id,source_course_version_id,source_version,source_language,target_language,creator_user_id,status,revision,translated_tree,created_at,updated_at`, id, expected, encoded, at.UTC()))
	if errors.Is(err, pgx.ErrNoRows) {
		_, getErr := r.Get(ctx, id)
		if getErr == nil {
			return translations.CourseTranslation{}, translations.ErrRevisionMismatch
		}
		return translations.CourseTranslation{}, storage(getErr)
	}
	if err != nil {
		return translations.CourseTranslation{}, storage(err)
	}
	return result, nil
}
func (r *Repository) Publish(ctx context.Context, id translations.TranslationID, input translations.TranslationPublication, at time.Time) (translations.TranslationPublication, error) {
	if r == nil || r.pool == nil || !validUUID(string(id)) || input.TranslationID != id || input.Source.Validate() != nil || input.TargetLanguage == "" || input.Revision < 1 || at.IsZero() || input.Tree.ValidateForPersistence() != nil {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return translations.TranslationPublication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanTranslation(tx.QueryRow(ctx, `SELECT id,source_course_id,source_course_version_id,source_version,source_language,target_language,creator_user_id,status,revision,translated_tree,created_at,updated_at FROM translations.course_translation WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if current.Source != input.Source || current.TargetLanguage != input.TargetLanguage || current.Revision != input.Revision {
		return translations.TranslationPublication{}, translations.ErrRevisionMismatch
	}
	encoded, err := json.Marshal(input.Tree)
	if err != nil {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	publication, err := scanPublication(tx.QueryRow(ctx, `INSERT INTO translations.translation_publication(translation_id,source_course_id,source_course_version_id,source_version,source_language,target_language,translation_revision,translated_tree,published_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
RETURNING id,translation_id,source_course_id,source_course_version_id,source_version,source_language,target_language,translation_revision,translated_tree,published_at`, id, input.Source.CourseID, input.Source.CourseVersionID, input.Source.Version.String(), input.Source.Language, input.TargetLanguage, input.Revision, encoded, at.UTC()))
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE translations.course_translation SET status='PUBLISHED',updated_at=$2 WHERE id=$1`, id, at.UTC()); err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return translations.TranslationPublication{}, err
	}
	return publication, nil
}
func (r *Repository) GetLatestPublication(ctx context.Context, sourceID courses.CourseVersionID, language courses.LanguageTag) (translations.TranslationPublication, error) {
	if r == nil || r.pool == nil || !validUUID(string(sourceID)) || language == "" {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	result, err := scanPublication(r.pool.QueryRow(ctx, `SELECT id,translation_id,source_course_id,source_course_version_id,source_version,source_language,target_language,translation_revision,translated_tree,published_at FROM translations.translation_publication WHERE source_course_version_id=$1 AND target_language=$2 ORDER BY translation_revision DESC,id DESC LIMIT 1`, sourceID, language))
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	return result, nil
}
func (r *Repository) ListLanguages(ctx context.Context, sourceID courses.CourseVersionID) ([]courses.LanguageTag, error) {
	if r == nil || r.pool == nil || !validUUID(string(sourceID)) {
		return nil, translations.ErrInvalidTranslation
	}
	rows, err := r.pool.Query(ctx, `SELECT target_language FROM translations.translation_publication WHERE source_course_version_id=$1 GROUP BY target_language ORDER BY target_language`, sourceID)
	if err != nil {
		return nil, storage(err)
	}
	defer rows.Close()
	var languages []courses.LanguageTag
	for rows.Next() {
		var language string
		if err := rows.Scan(&language); err != nil {
			return nil, err
		}
		normalized, err := courses.NormalizeLanguageTag(language)
		if err != nil || string(normalized) != language {
			return nil, translations.ErrInvalidTranslation
		}
		languages = append(languages, normalized)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return languages, nil
}

type scanner interface{ Scan(...any) error }

func scanTranslation(row scanner) (translations.CourseTranslation, error) {
	var t translations.CourseTranslation
	var version string
	var tree []byte
	err := row.Scan(&t.ID, &t.Source.CourseID, &t.Source.CourseVersionID, &version, &t.Source.Language, &t.TargetLanguage, &t.CreatorUserID, &t.Status, &t.Revision, &tree, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return translations.CourseTranslation{}, err
	}
	parsed, err := courses.ParseVersion(version)
	if err != nil {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	t.Source.Version = parsed
	if err := json.Unmarshal(tree, &t.Tree); err != nil {
		return translations.CourseTranslation{}, translations.ErrInvalidTranslation
	}
	if err := t.Validate(); err != nil {
		return translations.CourseTranslation{}, err
	}
	return t, nil
}
func scanPublication(row scanner) (translations.TranslationPublication, error) {
	var p translations.TranslationPublication
	var version string
	var tree []byte
	err := row.Scan(&p.ID, &p.TranslationID, &p.Source.CourseID, &p.Source.CourseVersionID, &version, &p.Source.Language, &p.TargetLanguage, &p.Revision, &tree, &p.PublishedAt)
	if err != nil {
		return translations.TranslationPublication{}, err
	}
	parsed, err := courses.ParseVersion(version)
	if err != nil {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	p.Source.Version = parsed
	if err := json.Unmarshal(tree, &p.Tree); err != nil {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	if err := p.Validate(); err != nil {
		return translations.TranslationPublication{}, err
	}
	return p, nil
}
func storage(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return translations.ErrTranslationNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return translations.ErrTranslationConflict
		case "23503":
			return translations.ErrTranslationNotFound
		case "23514", "23502", "22P02":
			return translations.ErrInvalidTranslation
		}
		return errors.New("translation storage failure")
	}
	return err
}
func validUUID(value string) bool { _, err := uuid.Parse(value); return err == nil }
