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
	"github.com/jackc/pgx/v5/pgtype"
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
func (r *Repository) ListBySourceVersion(ctx context.Context, id courses.CourseVersionID) ([]translations.CourseTranslation, error) {
	if r == nil || r.pool == nil || !validUUID(string(id)) {
		return nil, translations.ErrInvalidTranslation
	}
	rows, err := r.pool.Query(ctx, `SELECT id,source_course_id,source_course_version_id,source_version,source_language,target_language,creator_user_id,status,revision,translated_tree,created_at,updated_at FROM translations.course_translation WHERE source_course_version_id=$1 ORDER BY target_language,id`, id)
	if err != nil {
		return nil, storage(err)
	}
	defer rows.Close()
	result := make([]translations.CourseTranslation, 0)
	for rows.Next() {
		value, err := scanTranslation(rows)
		if err != nil {
			return nil, storage(err)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, storage(err)
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
	if r == nil || r.pool == nil || !validUUID(string(id)) || input.TranslationID != id || input.Source.Validate() != nil || input.TargetLanguage == "" || input.Revision < 1 || at.IsZero() || input.Tree.ValidateForPersistence() != nil || input.Provenance.Validate() != nil || input.Provenance.Origin != translations.PublicationOriginAuthoring || input.Provenance.Authoring.TranslationID != id || input.Provenance.Authoring.Revision != input.Revision {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return translations.TranslationPublication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockPublicationStream(ctx, tx, input.Source.CourseVersionID, input.TargetLanguage); err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	var importedExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM translations.translation_publication WHERE source_course_version_id=$1 AND target_language=$2 AND publication_origin='IMPORTED_PUBLICATION')`, input.Source.CourseVersionID, input.TargetLanguage).Scan(&importedExists); err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if importedExists {
		return translations.TranslationPublication{}, translations.ErrTranslationConflict
	}
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
	var publicationID translations.TranslationPublicationID
	err = tx.QueryRow(ctx, `INSERT INTO translations.translation_publication(translation_id,source_course_id,source_course_version_id,source_version,source_language,target_language,translation_revision,translated_tree,published_at,publication_origin)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'AUTHORING_PUBLICATION') RETURNING id`, id, input.Source.CourseID, input.Source.CourseVersionID, input.Source.Version.String(), input.Source.Language, input.TargetLanguage, input.Revision, encoded, at.UTC()).Scan(&publicationID)
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE translations.course_translation SET status='PUBLISHED',updated_at=$2 WHERE id=$1`, id, at.UTC()); err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	publication, err := getPublicationByID(ctx, tx, publicationID)
	if err != nil {
		return translations.TranslationPublication{}, err
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
	return getLatestPublication(ctx, r.pool, sourceID, language)
}

type publicationQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const publicationColumns = `p.id,p.translation_id,p.source_course_id,p.source_course_version_id,p.source_version,p.source_language,p.target_language,p.translation_revision,p.translated_tree,p.published_at,p.publication_origin,p.import_id,r.package_fingerprint,r.portable_source_course_id,r.portable_source_course_version_id,r.imported_at,r.source_version`

func getLatestPublication(ctx context.Context, q publicationQuerier, sourceID courses.CourseVersionID, language courses.LanguageTag) (translations.TranslationPublication, error) {
	result, err := scanPublication(q.QueryRow(ctx, `SELECT `+publicationColumns+` FROM translations.translation_publication p LEFT JOIN portability.import_record r ON r.id=p.import_id WHERE p.source_course_version_id=$1 AND p.target_language=$2 ORDER BY p.translation_revision DESC NULLS LAST,p.published_at DESC,p.id DESC LIMIT 1`, sourceID, language))
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	return result, nil
}

func lockPublicationStream(ctx context.Context, tx pgx.Tx, sourceID courses.CourseVersionID, language courses.LanguageTag) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text || ':' || $2::text, 0))`, sourceID, language)
	return err
}

// StoreImportedPublicationInTx writes a completed immutable translation using
// the caller's transaction. It creates no workspace and owns no tx lifecycle.
func (r *Repository) StoreImportedPublicationInTx(ctx context.Context, tx pgx.Tx, source courses.ImmutableCourseVersion, target courses.LanguageTag, tree translations.TranslationTree, provenance translations.ImportedPublicationProvenance, publishedAt time.Time) (translations.TranslationPublication, error) {
	if r == nil || tx == nil || source.ID == "" || source.CourseVersion.Status != courses.CourseVersionPublished || provenance.ImportedAt.IsZero() || publishedAt.IsZero() {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	binding := translations.SourceCourseVersion{CourseID: source.CourseVersion.CourseID, CourseVersionID: source.ID, Version: source.CourseVersion.Version, Language: source.CourseVersion.SourceLanguage}
	input := translations.TranslationPublication{Source: binding, TargetLanguage: target, Tree: tree, PublishedAt: publishedAt, Provenance: translations.PublicationProvenance{Origin: translations.PublicationOriginImported, Imported: &provenance}}
	if input.Provenance.Validate() != nil || translations.ValidateImportedPublication(source, target, tree) != nil || target == "" {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	if normalized, err := courses.NormalizeLanguageTag(string(target)); err != nil || normalized != target {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	var persistedCourseID, persistedVersionID string
	var persistedVersion, persistedLanguage, persistedStatus string
	err := tx.QueryRow(ctx, `SELECT course_id::text,id::text,version,source_language,status FROM courses.course_version WHERE id=$1`, source.ID).Scan(&persistedCourseID, &persistedVersionID, &persistedVersion, &persistedLanguage, &persistedStatus)
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if persistedCourseID != string(binding.CourseID) || persistedVersionID != string(binding.CourseVersionID) || persistedVersion != binding.Version.String() || persistedLanguage != string(binding.Language) || persistedStatus != "PUBLISHED" {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	var persistedFingerprint, portableCourseID, portableVersionID, importedVersion string
	var persistedImportedAt time.Time
	err = tx.QueryRow(ctx, `SELECT package_fingerprint,portable_source_course_id,portable_source_course_version_id,source_version,imported_at FROM portability.import_record WHERE id=$1`, provenance.ImportID).Scan(&persistedFingerprint, &portableCourseID, &portableVersionID, &importedVersion, &persistedImportedAt)
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if persistedFingerprint != provenance.PackageFingerprint || portableCourseID != provenance.PortableSourceCourseID || portableVersionID != provenance.PortableSourceCourseVersionID || importedVersion != binding.Version.String() || !persistedImportedAt.Equal(provenance.ImportedAt) {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	if err := lockPublicationStream(ctx, tx, source.ID, target); err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM translations.translation_publication WHERE source_course_version_id=$1 AND target_language=$2)`, source.ID, target).Scan(&exists); err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	if exists {
		return translations.TranslationPublication{}, translations.ErrTranslationConflict
	}
	encoded, err := json.Marshal(tree)
	if err != nil {
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
	}
	var id translations.TranslationPublicationID
	err = tx.QueryRow(ctx, `INSERT INTO translations.translation_publication(source_course_id,source_course_version_id,source_version,source_language,target_language,translated_tree,published_at,publication_origin,import_id) VALUES($1,$2,$3,$4,$5,$6,$7,'IMPORTED_PUBLICATION',$8) RETURNING id`, binding.CourseID, binding.CourseVersionID, binding.Version.String(), binding.Language, target, encoded, publishedAt.UTC(), provenance.ImportID).Scan(&id)
	if err != nil {
		return translations.TranslationPublication{}, storage(err)
	}
	return getPublicationByID(ctx, tx, id)
}

func getPublicationByID(ctx context.Context, q publicationQuerier, id translations.TranslationPublicationID) (translations.TranslationPublication, error) {
	result, err := scanPublication(q.QueryRow(ctx, `SELECT `+publicationColumns+` FROM translations.translation_publication p LEFT JOIN portability.import_record r ON r.id=p.import_id WHERE p.id=$1`, id))
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
	var workspaceID, importID pgtype.UUID
	var revision pgtype.Int8
	var origin string
	var fingerprint, portableCourseID, portableVersionID pgtype.Text
	var importedSourceVersion pgtype.Text
	var importedAt pgtype.Timestamptz
	err := row.Scan(&p.ID, &workspaceID, &p.Source.CourseID, &p.Source.CourseVersionID, &version, &p.Source.Language, &p.TargetLanguage, &revision, &tree, &p.PublishedAt, &origin, &importID, &fingerprint, &portableCourseID, &portableVersionID, &importedAt, &importedSourceVersion)
	if err != nil {
		return translations.TranslationPublication{}, err
	}
	switch translations.PublicationOrigin(origin) {
	case translations.PublicationOriginAuthoring:
		if !workspaceID.Valid || !revision.Valid || importID.Valid || fingerprint.Valid || portableCourseID.Valid || portableVersionID.Valid || importedAt.Valid || importedSourceVersion.Valid {
			return translations.TranslationPublication{}, translations.ErrInvalidTranslation
		}
		p.TranslationID = translations.TranslationID(workspaceID.String())
		p.Revision = revision.Int64
		p.Provenance = translations.PublicationProvenance{Origin: translations.PublicationOriginAuthoring, Authoring: &translations.AuthoringPublicationProvenance{TranslationID: p.TranslationID, Revision: p.Revision}}
	case translations.PublicationOriginImported:
		if workspaceID.Valid || revision.Valid || !importID.Valid || !fingerprint.Valid || !portableCourseID.Valid || !portableVersionID.Valid || !importedAt.Valid || !importedSourceVersion.Valid || importedSourceVersion.String != version {
			return translations.TranslationPublication{}, translations.ErrInvalidTranslation
		}
		p.Provenance = translations.PublicationProvenance{Origin: translations.PublicationOriginImported, Imported: &translations.ImportedPublicationProvenance{ImportID: importID.String(), PackageFingerprint: fingerprint.String, PortableSourceCourseID: portableCourseID.String, PortableSourceCourseVersionID: portableVersionID.String, ImportedAt: importedAt.Time.UTC()}}
	default:
		return translations.TranslationPublication{}, translations.ErrInvalidTranslation
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
