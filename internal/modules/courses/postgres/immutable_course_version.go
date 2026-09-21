package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/db/sqlc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ courses.ImmutableCourseVersionRepository = (*Repository)(nil)
var _ courses.PublishedCourseVersionRepository = (*Repository)(nil)

func (r *Repository) StoreImmutableCourseVersion(ctx context.Context, input courses.ImmutableCourseVersion) (courses.ImmutableCourseVersion, error) {
	if input.ValidateForPersistence() != nil {
		return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
	}
	if r == nil || r.begin == nil {
		return courses.ImmutableCourseVersion{}, errors.New("course transaction unavailable")
	}
	tx, err := r.begin.Begin(ctx)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	txRepository := &Repository{q: q}

	version, err := txRepository.CreateCourseVersion(ctx, input.CourseVersion)
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	versionID, err := uuid(string(version.ID))
	if err != nil {
		return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
	}
	if err := storePublicationProvenance(ctx, q, versionID, input.Provenance); err != nil {
		return courses.ImmutableCourseVersion{}, immutableStoreError(err)
	}
	for _, binding := range input.AssetBindings {
		assetKey, err := uuid(binding.AssetKey)
		if err != nil {
			return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
		}
		storageObjectID, err := uuid(binding.StorageObjectID)
		if err != nil {
			return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
		}
		if _, err := q.CreateCourseVersionAssetBinding(ctx, sqlc.CreateCourseVersionAssetBindingParams{
			CourseVersionID: versionID, AssetKey: assetKey, StorageObjectID: storageObjectID,
			OriginalFilename: binding.OriginalFilename, MediaType: binding.MediaType,
			ByteSize: binding.ByteSize, Sha256Digest: binding.SHA256Digest,
		}); err != nil {
			return courses.ImmutableCourseVersion{}, immutableStoreError(err)
		}
	}
	for _, binding := range input.AssessmentBindings {
		assessmentKey, err := uuid(binding.AssessmentKey)
		if err != nil {
			return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
		}
		definition, err := marshalPublishedAssessment(binding)
		if err != nil {
			return courses.ImmutableCourseVersion{}, err
		}
		if _, err := q.CreateCourseVersionAssessmentBinding(ctx, sqlc.CreateCourseVersionAssessmentBindingParams{CourseVersionID: versionID, AssessmentKey: assessmentKey, Definition: definition}); err != nil {
			return courses.ImmutableCourseVersion{}, immutableStoreError(err)
		}
	}

	lessonIDs := make(map[string]pgtype.UUID)
	for _, module := range input.Modules {
		sourceModuleID, err := uuid(module.SourceID)
		if err != nil {
			return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
		}
		moduleRow, err := q.CreateImmutableCourseVersionModule(ctx, sqlc.CreateImmutableCourseVersionModuleParams{
			CourseVersionID: versionID,
			SourceModuleID:  sourceModuleID,
			StableKey:       module.StableKey,
			Title:           module.Title,
			Description:     module.Description,
			Position:        int32(module.Position),
		})
		if err != nil {
			return courses.ImmutableCourseVersion{}, immutableStoreError(err)
		}
		for _, lesson := range module.Lessons {
			row, err := createImmutableLesson(ctx, q, versionID, moduleRow.ID, lesson)
			if err != nil {
				return courses.ImmutableCourseVersion{}, immutableStoreError(err)
			}
			lessonIDs[lesson.StableKey] = row.ID
		}
	}

	for _, module := range input.Modules {
		for _, lesson := range module.Lessons {
			for position, prerequisiteKey := range lesson.PrerequisiteStableKeys {
				_, err := q.CreateLessonPrerequisite(ctx, sqlc.CreateLessonPrerequisiteParams{
					CourseVersionID:      versionID,
					LessonID:             lessonIDs[lesson.StableKey],
					PrerequisiteLessonID: lessonIDs[prerequisiteKey],
					Position:             int32(position),
				})
				if err != nil {
					return courses.ImmutableCourseVersion{}, immutableStoreError(err)
				}
			}
		}
	}

	stored, err := getImmutableCourseVersion(ctx, q, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	return stored, nil
}

func (r *Repository) GetImmutableCourseVersion(ctx context.Context, id courses.CourseVersionID) (courses.ImmutableCourseVersion, error) {
	key, err := uuid(string(id))
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	return getImmutableCourseVersion(ctx, r.q, key)
}

func (r *Repository) GetImmutableCourseVersionByReviewID(ctx context.Context, reviewID string) (courses.ImmutableCourseVersion, error) {
	key, err := uuid(reviewID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	versionID, err := r.q.GetCourseVersionIDByReviewID(ctx, key)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	return getImmutableCourseVersion(ctx, r.q, versionID)
}

func (r *Repository) GetImmutableCourseVersionByCourseAndVersion(ctx context.Context, courseID courses.CourseID, version courses.Version) (courses.ImmutableCourseVersion, error) {
	courseKey, err := uuid(string(courseID))
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	row, err := r.q.GetCourseVersionByCourseAndVersion(ctx, sqlc.GetCourseVersionByCourseAndVersionParams{CourseID: courseKey, Version: version.String()})
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	return getImmutableCourseVersion(ctx, r.q, row.ID)
}

func (r *Repository) GetPublishedImmutableCourseVersionByCourseAndVersion(ctx context.Context, courseID courses.CourseID, version courses.Version) (courses.ImmutableCourseVersion, error) {
	courseKey, err := uuid(string(courseID))
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	versionID, err := r.q.GetPublishedCourseVersionIDByCourseAndVersion(ctx, sqlc.GetPublishedCourseVersionIDByCourseAndVersionParams{CourseID: courseKey, Version: version.String()})
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	return getImmutableCourseVersion(ctx, r.q, versionID)
}

func (r *Repository) GetPublishedAssetBindingByCourseAndVersionAndAssetKey(ctx context.Context, courseID courses.CourseID, version courses.Version, assetKey string) (courses.PublishedAssetBinding, error) {
	courseKey, err := uuid(string(courseID))
	if err != nil {
		return courses.PublishedAssetBinding{}, courses.ErrNotFound
	}
	assetID, err := uuid(assetKey)
	if err != nil {
		return courses.PublishedAssetBinding{}, courses.ErrNotFound
	}
	row, err := r.q.GetPublishedAssetBindingByCourseAndVersionAndAssetKey(ctx, sqlc.GetPublishedAssetBindingByCourseAndVersionAndAssetKeyParams{
		CourseID: courseKey,
		Version:  version.String(),
		AssetKey: assetID,
	})
	if err != nil {
		return courses.PublishedAssetBinding{}, storageError(err)
	}
	binding := courses.PublishedAssetBinding{
		AssetKey: row.AssetKey.String(), StorageObjectID: row.StorageObjectID.String(),
		OriginalFilename: row.OriginalFilename, MediaType: row.MediaType,
		ByteSize: row.ByteSize, SHA256Digest: row.Sha256Digest,
	}
	if err := binding.Validate(); err != nil {
		return courses.PublishedAssetBinding{}, courses.ErrInvalidImmutableCourseVersion
	}
	return binding, nil
}

func (r *Repository) GetLatestPublishedImmutableCourseVersion(ctx context.Context, courseID courses.CourseID) (courses.ImmutableCourseVersion, error) {
	courseKey, err := uuid(string(courseID))
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	versionID, err := r.q.GetLatestPublishedCourseVersionID(ctx, courseKey)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	return getImmutableCourseVersion(ctx, r.q, versionID)
}

func storePublicationProvenance(ctx context.Context, q *sqlc.Queries, courseVersionID pgtype.UUID, provenance courses.CourseVersionProvenance) error {
	reviewID, err := uuid(provenance.ReviewID)
	if err != nil {
		return courses.ErrInvalidImmutableCourseVersion
	}
	draftID, err := uuid(provenance.DraftID)
	if err != nil {
		return courses.ErrInvalidImmutableCourseVersion
	}
	submitterID, err := uuid(provenance.SubmittedByUserID)
	if err != nil {
		return courses.ErrInvalidImmutableCourseVersion
	}
	approverID, err := uuid(provenance.ApprovedByUserID)
	if err != nil || provenance.ApprovedAt == nil {
		return courses.ErrInvalidImmutableCourseVersion
	}
	publisherID, err := uuid(provenance.PublishedByUserID)
	if err != nil {
		return courses.ErrInvalidImmutableCourseVersion
	}
	_, err = q.CreateCourseVersionPublicationProvenance(ctx, sqlc.CreateCourseVersionPublicationProvenanceParams{
		CourseVersionID:       courseVersionID,
		ReviewID:              reviewID,
		ReviewRevision:        provenance.ReviewRevision,
		DraftID:               draftID,
		DraftRevision:         provenance.DraftRevision,
		SnapshotSchemaVersion: int32(provenance.SnapshotSchemaVersion),
		SubmittedByUserID:     submitterID,
		SubmittedAt:           pgtype.Timestamptz{Time: provenance.SubmittedAt, Valid: true},
		ApprovedByUserID:      approverID,
		ApprovedAt:            pgtype.Timestamptz{Time: *provenance.ApprovedAt, Valid: true},
		PublishedByUserID:     publisherID,
	})
	return err
}

func createImmutableLesson(ctx context.Context, q *sqlc.Queries, versionID, moduleID pgtype.UUID, lesson courses.ImmutableCourseVersionLesson) (sqlc.CoursesLesson, error) {
	sourceLessonID, err := uuid(lesson.SourceID)
	if err != nil {
		return sqlc.CoursesLesson{}, courses.ErrInvalidImmutableCourseVersion
	}
	objectives, err := json.Marshal(lesson.LearningObjectives)
	if err != nil {
		return sqlc.CoursesLesson{}, courses.ErrInvalidImmutableCourseVersion
	}
	content, err := courses.MarshalLessonContent(lesson.Content)
	if err != nil {
		return sqlc.CoursesLesson{}, courses.ErrInvalidImmutableCourseVersion
	}
	params := sqlc.CreateImmutableCourseVersionLessonParams{
		CourseVersionID:    versionID,
		ModuleID:           moduleID,
		SourceLessonID:     sourceLessonID,
		StableKey:          lesson.StableKey,
		Title:              lesson.Title,
		Description:        lesson.Description,
		LearningObjectives: objectives,
		Position:           int32(lesson.Position),
		Content:            content,
	}
	if lesson.EstimatedDurationMinutes != nil {
		params.EstimatedDurationMinutes = pgtype.Int4{Int32: int32(*lesson.EstimatedDurationMinutes), Valid: true}
	}
	return q.CreateImmutableCourseVersionLesson(ctx, params)
}

func getImmutableCourseVersion(ctx context.Context, q *sqlc.Queries, versionID pgtype.UUID) (courses.ImmutableCourseVersion, error) {
	versionRow, err := q.GetCourseVersion(ctx, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	version, err := mapCourseVersion(versionRow)
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	provenanceRow, err := q.GetCourseVersionPublicationProvenance(ctx, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	result := courses.ImmutableCourseVersion{
		ID: version.ID,
		CourseVersion: courses.CourseVersionInput{
			CourseID:           version.CourseID,
			Version:            version.Version,
			Status:             version.Status,
			Title:              version.Title,
			Description:        version.Description,
			LearningObjectives: version.LearningObjectives,
			SourceLanguage:     version.SourceLanguage,
			Changelog:          version.Changelog,
			License:            version.License,
			Attribution:        version.Attribution,
			PublishedAt:        version.PublishedAt.UTC(),
		},
		Provenance: courses.CourseVersionProvenance{
			ReviewID:              provenanceRow.ReviewID.String(),
			ReviewRevision:        provenanceRow.ReviewRevision,
			DraftID:               provenanceRow.DraftID.String(),
			DraftRevision:         provenanceRow.DraftRevision,
			SnapshotSchemaVersion: int(provenanceRow.SnapshotSchemaVersion),
			SubmittedByUserID:     provenanceRow.SubmittedByUserID.String(),
			SubmittedAt:           provenanceRow.SubmittedAt.Time.UTC(),
			ApprovedByUserID:      provenanceRow.ApprovedByUserID.String(),
			ApprovedAt:            cloneTime(provenanceRow.ApprovedAt.Time.UTC()),
			PublishedByUserID:     provenanceRow.PublishedByUserID.String(),
		},
		Modules:            []courses.ImmutableCourseVersionModule{},
		AssetBindings:      []courses.PublishedAssetBinding{},
		AssessmentBindings: []courses.PublishedAssessmentBinding{},
	}
	bindingRows, err := q.ListCourseVersionAssetBindings(ctx, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	for _, row := range bindingRows {
		result.AssetBindings = append(result.AssetBindings, courses.PublishedAssetBinding{
			AssetKey: row.AssetKey.String(), StorageObjectID: row.StorageObjectID.String(),
			OriginalFilename: row.OriginalFilename, MediaType: row.MediaType,
			ByteSize: row.ByteSize, SHA256Digest: row.Sha256Digest,
		})
	}
	assessmentRows, err := q.ListCourseVersionAssessmentBindings(ctx, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	for _, row := range assessmentRows {
		binding, err := unmarshalPublishedAssessment(row.AssessmentKey.String(), row.Definition)
		if err != nil {
			return courses.ImmutableCourseVersion{}, err
		}
		result.AssessmentBindings = append(result.AssessmentBindings, binding)
	}
	moduleRows, err := q.ListModulesForCourseVersion(ctx, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	moduleIndexes := make(map[string]int, len(moduleRows))
	for _, moduleRow := range moduleRows {
		if !moduleRow.SourceModuleID.Valid {
			return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
		}
		module := courses.ImmutableCourseVersionModule{
			ID:          courses.ModuleID(moduleRow.ID.String()),
			SourceID:    moduleRow.SourceModuleID.String(),
			StableKey:   moduleRow.StableKey,
			Title:       moduleRow.Title,
			Description: moduleRow.Description,
			Position:    int(moduleRow.Position),
			Lessons:     []courses.ImmutableCourseVersionLesson{},
		}
		result.Modules = append(result.Modules, module)
		moduleIndexes[moduleRow.ID.String()] = len(result.Modules) - 1
	}
	lessonRows, err := q.ListLessonsForCourseVersion(ctx, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	prerequisiteRows, err := q.ListLessonPrerequisitesForCourseVersion(ctx, versionID)
	if err != nil {
		return courses.ImmutableCourseVersion{}, storageError(err)
	}
	prerequisites := make(map[string][]string, len(lessonRows))
	for _, row := range prerequisiteRows {
		prerequisites[row.LessonID.String()] = append(prerequisites[row.LessonID.String()], row.PrerequisiteStableKey)
	}
	for _, lessonRow := range lessonRows {
		if !lessonRow.SourceLessonID.Valid {
			return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
		}
		moduleIndex, ok := moduleIndexes[lessonRow.ModuleID.String()]
		if !ok {
			return courses.ImmutableCourseVersion{}, courses.ErrInvalidImmutableCourseVersion
		}
		lesson, err := mapLesson(lessonRow)
		if err != nil {
			return courses.ImmutableCourseVersion{}, err
		}
		result.Modules[moduleIndex].Lessons = append(result.Modules[moduleIndex].Lessons, courses.ImmutableCourseVersionLesson{
			ID:                       lesson.ID,
			SourceID:                 lessonRow.SourceLessonID.String(),
			StableKey:                lesson.StableKey,
			Title:                    lesson.Title,
			Description:              lesson.Description,
			LearningObjectives:       lesson.LearningObjectives,
			EstimatedDurationMinutes: lesson.EstimatedDurationMinutes,
			Position:                 lesson.Position,
			PrerequisiteStableKeys:   append([]string{}, prerequisites[lessonRow.ID.String()]...),
			Content:                  lesson.Content,
		})
	}
	return result, nil
}

func immutableStoreError(err error) error {
	if err == nil || errors.Is(err, courses.ErrInvalidImmutableCourseVersion) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" && (pgErr.ConstraintName == "course_version_unique" || pgErr.ConstraintName == "course_version_publication_review_unique") {
			return courses.ErrCourseVersionAlreadyExists
		}
		if pgErr.Code == "23505" || pgErr.Code == "23514" || pgErr.Code == "23502" || pgErr.Code == "22P02" {
			return courses.ErrInvalidImmutableCourseVersion
		}
	}
	return storageError(err)
}

func cloneTime(value time.Time) *time.Time {
	copy := value
	return &copy
}
