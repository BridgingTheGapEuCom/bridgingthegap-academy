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

type transactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Repository implements Courses-owned persistence contracts using only Courses tables.
type Repository struct {
	q     *sqlc.Queries
	begin transactionBeginner
}

var _ courses.Repository = (*Repository)(nil)
var _ courses.PublishedCatalogRepository = (*Repository)(nil)

func New(db sqlc.DBTX) *Repository {
	r := &Repository{q: sqlc.New(db)}
	if begin, ok := db.(transactionBeginner); ok {
		r.begin = begin
	}
	return r
}

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
			if pgErr.ConstraintName == "course_version_unique" || pgErr.ConstraintName == "course_version_publication_review_unique" {
				return courses.ErrCourseVersionAlreadyExists
			}
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

func mapModule(row sqlc.CoursesModule) courses.Module {
	return courses.Module{
		ID:              courses.ModuleID(row.ID.String()),
		CourseVersionID: courses.CourseVersionID(row.CourseVersionID.String()),
		StableKey:       row.StableKey,
		Title:           row.Title,
		Description:     row.Description,
		Position:        int(row.Position),
		CreatedAt:       row.CreatedAt.Time,
	}
}

func mapLesson(row sqlc.CoursesLesson) (courses.Lesson, error) {
	var objectives []string
	if err := json.Unmarshal(row.LearningObjectives, &objectives); err != nil {
		return courses.Lesson{}, errors.New("invalid stored lesson learning objectives")
	}
	content, err := courses.ParseLessonContent(row.Content)
	if err != nil {
		return courses.Lesson{}, errors.New("invalid stored lesson content")
	}
	var duration *int
	if row.EstimatedDurationMinutes.Valid {
		value := int(row.EstimatedDurationMinutes.Int32)
		duration = &value
	}
	lesson := courses.Lesson{
		ID:                       courses.LessonID(row.ID.String()),
		CourseVersionID:          courses.CourseVersionID(row.CourseVersionID.String()),
		ModuleID:                 courses.ModuleID(row.ModuleID.String()),
		StableKey:                row.StableKey,
		Title:                    row.Title,
		Description:              row.Description,
		LearningObjectives:       objectives,
		EstimatedDurationMinutes: duration,
		Position:                 int(row.Position),
		Content:                  content,
		CreatedAt:                row.CreatedAt.Time,
	}
	if err := (courses.LessonInput{
		CourseVersionID:          lesson.CourseVersionID,
		ModuleID:                 lesson.ModuleID,
		StableKey:                lesson.StableKey,
		Title:                    lesson.Title,
		Description:              lesson.Description,
		LearningObjectives:       lesson.LearningObjectives,
		EstimatedDurationMinutes: lesson.EstimatedDurationMinutes,
		Position:                 lesson.Position,
		Content:                  lesson.Content,
	}).Validate(); err != nil {
		return courses.Lesson{}, errors.New("invalid stored lesson")
	}
	return lesson, nil
}

func mapLessonSummary(row sqlc.ListLessonSummariesForCourseVersionRow) (courses.LessonSummary, error) {
	var objectives []string
	if err := json.Unmarshal(row.LearningObjectives, &objectives); err != nil {
		return courses.LessonSummary{}, errors.New("invalid stored lesson learning objectives")
	}
	var duration *int
	if row.EstimatedDurationMinutes.Valid {
		value := int(row.EstimatedDurationMinutes.Int32)
		duration = &value
	}
	summary := courses.LessonSummary{
		ID:                       courses.LessonID(row.ID.String()),
		ModuleID:                 courses.ModuleID(row.ModuleID.String()),
		StableKey:                row.StableKey,
		Title:                    row.Title,
		Description:              row.Description,
		LearningObjectives:       objectives,
		EstimatedDurationMinutes: duration,
		Position:                 int(row.Position),
	}
	if err := (courses.LessonInput{
		CourseVersionID:          courses.CourseVersionID(row.CourseVersionID.String()),
		ModuleID:                 summary.ModuleID,
		StableKey:                summary.StableKey,
		Title:                    summary.Title,
		Description:              summary.Description,
		LearningObjectives:       summary.LearningObjectives,
		EstimatedDurationMinutes: summary.EstimatedDurationMinutes,
		Position:                 summary.Position,
	}).ValidateMetadata(); err != nil {
		return courses.LessonSummary{}, errors.New("invalid stored lesson metadata")
	}
	return summary, nil
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

func (r *Repository) ListPublishedCourseVersions(ctx context.Context) ([]courses.CourseVersion, error) {
	rows, err := r.q.ListPublishedCourseVersions(ctx)
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

func (r *Repository) ListLatestPublishedCourseVersions(ctx context.Context, query courses.PublishedCatalogQuery) ([]courses.CourseVersion, error) {
	params := sqlc.ListLatestPublishedCourseVersionsParams{
		Language: catalogLanguage(query.Language), Offset: int32(query.Offset), Limit: int32(query.Limit),
	}
	rows, err := r.q.ListLatestPublishedCourseVersions(ctx, params)
	if err != nil {
		return nil, storageError(err)
	}
	versions := make([]courses.CourseVersion, 0, len(rows))
	for _, row := range rows {
		version, err := mapCourseVersion(row)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, nil
}

func (r *Repository) CountLatestPublishedCourses(ctx context.Context, language *courses.LanguageTag) (int, error) {
	count, err := r.q.CountLatestPublishedCourses(ctx, catalogLanguage(language))
	if err != nil {
		return 0, storageError(err)
	}
	if count < 0 || count > int64(^uint(0)>>1) {
		return 0, errors.New("invalid published catalog count")
	}
	return int(count), nil
}

func catalogLanguage(language *courses.LanguageTag) pgtype.Text {
	if language == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*language), Valid: true}
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

func (r *Repository) CreateModule(ctx context.Context, input courses.ModuleInput) (courses.Module, error) {
	if err := input.Validate(); err != nil {
		return courses.Module{}, err
	}
	versionID, err := uuid(string(input.CourseVersionID))
	if err != nil {
		return courses.Module{}, err
	}
	stableKey, err := courses.NormalizeStructureKey(input.StableKey)
	if err != nil {
		return courses.Module{}, err
	}
	row, err := r.q.CreateModule(ctx, sqlc.CreateModuleParams{
		CourseVersionID: versionID,
		StableKey:       stableKey,
		Title:           input.Title,
		Description:     input.Description,
		Position:        int32(input.Position),
	})
	if err != nil {
		return courses.Module{}, storageError(err)
	}
	return mapModule(row), nil
}

func (r *Repository) GetModule(ctx context.Context, id courses.ModuleID) (courses.Module, error) {
	key, err := uuid(string(id))
	if err != nil {
		return courses.Module{}, err
	}
	row, err := r.q.GetModule(ctx, key)
	if err != nil {
		return courses.Module{}, storageError(err)
	}
	return mapModule(row), nil
}

func (r *Repository) GetModuleByCourseVersionAndKey(ctx context.Context, courseVersionID courses.CourseVersionID, stableKey string) (courses.Module, error) {
	versionID, err := uuid(string(courseVersionID))
	if err != nil {
		return courses.Module{}, err
	}
	stableKey, err = courses.NormalizeStructureKey(stableKey)
	if err != nil {
		return courses.Module{}, err
	}
	row, err := r.q.GetModuleByCourseVersionAndKey(ctx, sqlc.GetModuleByCourseVersionAndKeyParams{CourseVersionID: versionID, StableKey: stableKey})
	if err != nil {
		return courses.Module{}, storageError(err)
	}
	return mapModule(row), nil
}

func (r *Repository) ListModulesForCourseVersion(ctx context.Context, courseVersionID courses.CourseVersionID) ([]courses.Module, error) {
	versionID, err := uuid(string(courseVersionID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListModulesForCourseVersion(ctx, versionID)
	if err != nil {
		return nil, storageError(err)
	}
	modules := make([]courses.Module, 0, len(rows))
	for _, row := range rows {
		modules = append(modules, mapModule(row))
	}
	return modules, nil
}

func (r *Repository) CreateLesson(ctx context.Context, input courses.LessonInput) (courses.Lesson, error) {
	if err := input.Validate(); err != nil {
		return courses.Lesson{}, err
	}
	versionID, err := uuid(string(input.CourseVersionID))
	if err != nil {
		return courses.Lesson{}, err
	}
	moduleID, err := uuid(string(input.ModuleID))
	if err != nil {
		return courses.Lesson{}, err
	}
	stableKey, err := courses.NormalizeStructureKey(input.StableKey)
	if err != nil {
		return courses.Lesson{}, err
	}
	objectives, err := json.Marshal(input.LearningObjectives)
	if err != nil {
		return courses.Lesson{}, errors.New("encode lesson learning objectives")
	}
	content, err := courses.MarshalLessonContent(input.Content)
	if err != nil {
		return courses.Lesson{}, err
	}
	params := sqlc.CreateLessonParams{
		CourseVersionID:    versionID,
		ModuleID:           moduleID,
		StableKey:          stableKey,
		Title:              input.Title,
		Description:        input.Description,
		LearningObjectives: objectives,
		Position:           int32(input.Position),
		Content:            content,
	}
	if input.EstimatedDurationMinutes != nil {
		params.EstimatedDurationMinutes = pgtype.Int4{Int32: int32(*input.EstimatedDurationMinutes), Valid: true}
	}
	row, err := r.q.CreateLesson(ctx, params)
	if err != nil {
		return courses.Lesson{}, storageError(err)
	}
	return mapLesson(row)
}

func (r *Repository) GetLesson(ctx context.Context, id courses.LessonID) (courses.Lesson, error) {
	key, err := uuid(string(id))
	if err != nil {
		return courses.Lesson{}, err
	}
	row, err := r.q.GetLesson(ctx, key)
	if err != nil {
		return courses.Lesson{}, storageError(err)
	}
	return mapLesson(row)
}

func (r *Repository) GetLessonByCourseVersionAndKey(ctx context.Context, courseVersionID courses.CourseVersionID, stableKey string) (courses.Lesson, error) {
	versionID, err := uuid(string(courseVersionID))
	if err != nil {
		return courses.Lesson{}, err
	}
	stableKey, err = courses.NormalizeStructureKey(stableKey)
	if err != nil {
		return courses.Lesson{}, err
	}
	row, err := r.q.GetLessonByCourseVersionAndKey(ctx, sqlc.GetLessonByCourseVersionAndKeyParams{CourseVersionID: versionID, StableKey: stableKey})
	if err != nil {
		return courses.Lesson{}, storageError(err)
	}
	return mapLesson(row)
}

func (r *Repository) ListLessonsForModule(ctx context.Context, moduleID courses.ModuleID) ([]courses.Lesson, error) {
	key, err := uuid(string(moduleID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListLessonsForModule(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	lessons := make([]courses.Lesson, 0, len(rows))
	for _, row := range rows {
		lesson, err := mapLesson(row)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, lesson)
	}
	return lessons, nil
}

func (r *Repository) ListLessonSummariesForCourseVersion(ctx context.Context, courseVersionID courses.CourseVersionID) ([]courses.LessonSummary, error) {
	key, err := uuid(string(courseVersionID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListLessonSummariesForCourseVersion(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	lessons := make([]courses.LessonSummary, 0, len(rows))
	for _, row := range rows {
		lesson, err := mapLessonSummary(row)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, lesson)
	}
	return lessons, nil
}

func (r *Repository) CreateLessonPrerequisite(ctx context.Context, input courses.LessonPrerequisiteInput) (courses.LessonPrerequisite, error) {
	if err := input.Validate(); err != nil {
		return courses.LessonPrerequisite{}, err
	}
	lessonID, err := uuid(string(input.LessonID))
	if err != nil {
		return courses.LessonPrerequisite{}, err
	}
	versionID, err := uuid(string(input.CourseVersionID))
	if err != nil {
		return courses.LessonPrerequisite{}, err
	}
	source, err := r.q.GetLesson(ctx, lessonID)
	if err != nil {
		return courses.LessonPrerequisite{}, storageError(err)
	}
	if source.CourseVersionID != versionID {
		return courses.LessonPrerequisite{}, courses.ErrInvalidLessonPrerequisite
	}
	stableKey, err := courses.NormalizeStructureKey(input.PrerequisiteStableKey)
	if err != nil {
		return courses.LessonPrerequisite{}, err
	}
	if source.StableKey == stableKey {
		return courses.LessonPrerequisite{}, courses.ErrInvalidLessonPrerequisite
	}
	target, err := r.q.GetLessonByCourseVersionAndKey(ctx, sqlc.GetLessonByCourseVersionAndKeyParams{CourseVersionID: versionID, StableKey: stableKey})
	if err != nil {
		return courses.LessonPrerequisite{}, storageError(err)
	}
	row, err := r.q.CreateLessonPrerequisite(ctx, sqlc.CreateLessonPrerequisiteParams{
		CourseVersionID:      versionID,
		LessonID:             lessonID,
		PrerequisiteLessonID: target.ID,
		Position:             int32(input.Position),
	})
	if err != nil {
		return courses.LessonPrerequisite{}, storageError(err)
	}
	return courses.LessonPrerequisite{
		CourseVersionID:       courses.CourseVersionID(row.CourseVersionID.String()),
		LessonID:              courses.LessonID(row.LessonID.String()),
		PrerequisiteLessonID:  courses.LessonID(row.PrerequisiteLessonID.String()),
		PrerequisiteStableKey: stableKey,
		Position:              int(row.Position),
	}, nil
}

func (r *Repository) ListLessonPrerequisites(ctx context.Context, lessonID courses.LessonID) ([]courses.LessonPrerequisite, error) {
	key, err := uuid(string(lessonID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListLessonPrerequisites(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	prerequisites := make([]courses.LessonPrerequisite, 0, len(rows))
	for _, row := range rows {
		prerequisites = append(prerequisites, courses.LessonPrerequisite{
			CourseVersionID:       courses.CourseVersionID(row.CourseVersionID.String()),
			LessonID:              courses.LessonID(row.LessonID.String()),
			PrerequisiteLessonID:  courses.LessonID(row.PrerequisiteLessonID.String()),
			PrerequisiteStableKey: row.PrerequisiteStableKey,
			Position:              int(row.Position),
		})
	}
	return prerequisites, nil
}

func (r *Repository) ListLessonPrerequisitesForCourseVersion(ctx context.Context, courseVersionID courses.CourseVersionID) ([]courses.LessonPrerequisite, error) {
	key, err := uuid(string(courseVersionID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListLessonPrerequisitesForCourseVersion(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	prerequisites := make([]courses.LessonPrerequisite, 0, len(rows))
	for _, row := range rows {
		prerequisites = append(prerequisites, courses.LessonPrerequisite{
			CourseVersionID:       courses.CourseVersionID(row.CourseVersionID.String()),
			LessonID:              courses.LessonID(row.LessonID.String()),
			PrerequisiteLessonID:  courses.LessonID(row.PrerequisiteLessonID.String()),
			PrerequisiteStableKey: row.PrerequisiteStableKey,
			Position:              int(row.Position),
		})
	}
	return prerequisites, nil
}
