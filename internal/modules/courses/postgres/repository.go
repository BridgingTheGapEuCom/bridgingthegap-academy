package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository implements Courses-owned persistence contracts using only Courses tables.
type Repository struct{ q *sqlc.Queries }

var _ courses.Repository = (*Repository)(nil)

func New(db sqlc.DBTX) *Repository { return &Repository{q: sqlc.New(db)} }

func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, errors.New("invalid course identifier")
	}
	return id, nil
}

func storageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return courses.ErrNotFound
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return courses.ErrConflict
		case "23503":
			return courses.ErrNotFound
		case "23514", "23502", "22P02":
			return errors.New("course constraint violation")
		}
		return fmt.Errorf("course storage failure (SQLSTATE %s)", pgErr.Code)
	}
	return errors.New("course storage failure")
}

func mapCourse(row sqlc.CoursesCourse) courses.Course {
	return courses.Course{ID: courses.CourseID(row.ID.String()), Slug: row.Slug, CreatedAt: row.CreatedAt.Time}
}

func mapCourseVersion(row sqlc.CoursesCourseVersion) (courses.CourseVersion, error) {
	version, err := courses.ParseVersion(row.Version)
	if err != nil {
		return courses.CourseVersion{}, err
	}
	status, err := courses.ParseCourseVersionStatus(row.Status)
	if err != nil {
		return courses.CourseVersion{}, err
	}
	language, err := courses.NormalizeLanguageTag(row.SourceLanguage)
	if err != nil {
		return courses.CourseVersion{}, err
	}
	var objectives []string
	if err := json.Unmarshal(row.LearningObjectives, &objectives); err != nil {
		return courses.CourseVersion{}, errors.New("invalid stored learning objectives")
	}
	var attribution []courses.ContributorSnapshot
	if err := json.Unmarshal(row.Attribution, &attribution); err != nil {
		return courses.CourseVersion{}, errors.New("invalid stored contributor attribution")
	}
	license := courses.ContentLicense{
		Kind:        courses.ContentLicenseKind(row.LicenseKind),
		DisplayName: row.LicenseDisplayName,
	}
	if row.LicenseIdentifier.Valid {
		license.Identifier = row.LicenseIdentifier.String
	}
	if row.LicenseUrl.Valid {
		license.URL = row.LicenseUrl.String
	}
	if row.LicenseCustomText.Valid {
		license.CustomText = row.LicenseCustomText.String
	}
	input := courses.CourseVersionInput{
		CourseID:           courses.CourseID(row.CourseID.String()),
		Version:            version,
		Status:             status,
		Title:              row.Title,
		Description:        row.Description,
		LearningObjectives: objectives,
		SourceLanguage:     language,
		Changelog:          row.Changelog,
		License:            license,
		Attribution:        attribution,
		PublishedAt:        row.PublishedAt.Time,
	}
	if err := input.Validate(); err != nil {
		return courses.CourseVersion{}, errors.New("invalid stored course version")
	}
	return courses.CourseVersion{
		ID:                 courses.CourseVersionID(row.ID.String()),
		CourseID:           input.CourseID,
		Version:            version,
		Status:             status,
		Title:              row.Title,
		Description:        row.Description,
		LearningObjectives: objectives,
		SourceLanguage:     language,
		Changelog:          row.Changelog,
		License:            license,
		Attribution:        attribution,
		CreatedAt:          row.CreatedAt.Time,
		PublishedAt:        row.PublishedAt.Time,
	}, nil
}

func (r *Repository) CreateCourse(ctx context.Context, slug string) (courses.Course, error) {
	normalized, err := courses.NormalizeSlug(slug)
	if err != nil {
		return courses.Course{}, err
	}
	row, err := r.q.CreateCourse(ctx, normalized)
	if err != nil {
		return courses.Course{}, storageError(err)
	}
	return mapCourse(row), nil
}

func (r *Repository) GetCourse(ctx context.Context, id courses.CourseID) (courses.Course, error) {
	key, err := uuid(string(id))
	if err != nil {
		return courses.Course{}, err
	}
	row, err := r.q.GetCourse(ctx, key)
	if err != nil {
		return courses.Course{}, storageError(err)
	}
	return mapCourse(row), nil
}

func (r *Repository) GetCourseBySlug(ctx context.Context, slug string) (courses.Course, error) {
	normalized, err := courses.NormalizeSlug(slug)
	if err != nil {
		return courses.Course{}, err
	}
	row, err := r.q.GetCourseBySlug(ctx, normalized)
	if err != nil {
		return courses.Course{}, storageError(err)
	}
	return mapCourse(row), nil
}

func (r *Repository) ListCourses(ctx context.Context) ([]courses.Course, error) {
	rows, err := r.q.ListCourses(ctx)
	if err != nil {
		return nil, storageError(err)
	}
	result := make([]courses.Course, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapCourse(row))
	}
	return result, nil
}

func (r *Repository) CreateCourseVersion(ctx context.Context, input courses.CourseVersionInput) (courses.CourseVersion, error) {
	if err := input.Validate(); err != nil {
		return courses.CourseVersion{}, err
	}
	courseID, err := uuid(string(input.CourseID))
	if err != nil {
		return courses.CourseVersion{}, err
	}
	language, err := courses.NormalizeLanguageTag(string(input.SourceLanguage))
	if err != nil {
		return courses.CourseVersion{}, err
	}
	objectives, err := json.Marshal(input.LearningObjectives)
	if err != nil {
		return courses.CourseVersion{}, errors.New("encode learning objectives")
	}
	attribution, err := json.Marshal(input.Attribution)
	if err != nil {
		return courses.CourseVersion{}, errors.New("encode contributor attribution")
	}
	params := sqlc.CreateCourseVersionParams{
		CourseID:           courseID,
		Version:            input.Version.String(),
		Status:             string(input.Status),
		Title:              input.Title,
		Description:        input.Description,
		LearningObjectives: objectives,
		SourceLanguage:     string(language),
		Changelog:          input.Changelog,
		LicenseKind:        string(input.License.Kind),
		LicenseDisplayName: input.License.DisplayName,
		Attribution:        attribution,
		PublishedAt:        pgtype.Timestamptz{Time: input.PublishedAt, Valid: true},
	}
	if input.License.Identifier != "" {
		params.LicenseIdentifier = pgtype.Text{String: input.License.Identifier, Valid: true}
	}
	if input.License.URL != "" {
		params.LicenseUrl = pgtype.Text{String: input.License.URL, Valid: true}
	}
	if input.License.CustomText != "" {
		params.LicenseCustomText = pgtype.Text{String: input.License.CustomText, Valid: true}
	}
	row, err := r.q.CreateCourseVersion(ctx, params)
	if err != nil {
		return courses.CourseVersion{}, storageError(err)
	}
	return mapCourseVersion(row)
}

func (r *Repository) GetCourseVersion(ctx context.Context, id courses.CourseVersionID) (courses.CourseVersion, error) {
	key, err := uuid(string(id))
	if err != nil {
		return courses.CourseVersion{}, err
	}
	row, err := r.q.GetCourseVersion(ctx, key)
	if err != nil {
		return courses.CourseVersion{}, storageError(err)
	}
	return mapCourseVersion(row)
}

func (r *Repository) GetCourseVersionByCourseAndVersion(ctx context.Context, courseID courses.CourseID, version courses.Version) (courses.CourseVersion, error) {
	if !version.Valid() {
		return courses.CourseVersion{}, errors.New("invalid course version")
	}
	key, err := uuid(string(courseID))
	if err != nil {
		return courses.CourseVersion{}, err
	}
	row, err := r.q.GetCourseVersionByCourseAndVersion(ctx, sqlc.GetCourseVersionByCourseAndVersionParams{CourseID: key, Version: version.String()})
	if err != nil {
		return courses.CourseVersion{}, storageError(err)
	}
	return mapCourseVersion(row)
}

func (r *Repository) ListCourseVersions(ctx context.Context, courseID courses.CourseID) ([]courses.CourseVersion, error) {
	key, err := uuid(string(courseID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListCourseVersions(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	result := make([]courses.CourseVersion, 0, len(rows))
	for _, row := range rows {
		version, err := mapCourseVersion(row)
		if err != nil {
			return nil, err
		}
		result = append(result, version)
	}
	return result, nil
}

func (r *Repository) TransitionCourseVersionStatus(ctx context.Context, id courses.CourseVersionID, current, next courses.CourseVersionStatus) (courses.CourseVersion, error) {
	if _, err := courses.ParseCourseVersionStatus(string(current)); err != nil {
		return courses.CourseVersion{}, err
	}
	if _, err := courses.ParseCourseVersionStatus(string(next)); err != nil {
		return courses.CourseVersion{}, err
	}
	if !current.CanTransitionTo(next) {
		return courses.CourseVersion{}, courses.ErrInvalidStatusTransition
	}
	key, err := uuid(string(id))
	if err != nil {
		return courses.CourseVersion{}, err
	}
	row, err := r.q.TransitionCourseVersionStatus(ctx, sqlc.TransitionCourseVersionStatusParams{ID: key, Status: string(current), Status_2: string(next)})
	if err != nil {
		return courses.CourseVersion{}, storageError(err)
	}
	return mapCourseVersion(row)
}
