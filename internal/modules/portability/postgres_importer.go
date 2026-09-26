package portability

import (
	"bytes"
	"context"
	"errors"
	"sort"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	assetspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursessqlc "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/db/sqlc"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
	translationspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresImportRepository is the platform-facing persistence implementation
// for already-validated package plans. It owns the outer transaction but keeps
// Course, Asset, and Translation writes in their owning modules.
type PostgresImportRepository struct {
	pool          *pgxpool.Pool
	storage       assets.BinaryStorage
	assetMaxBytes int64
}

var _ ImportRepository = (*PostgresImportRepository)(nil)

func NewPostgresImportRepository(pool *pgxpool.Pool, storage assets.BinaryStorage, assetMaxBytes int64) (*PostgresImportRepository, error) {
	if pool == nil || storage == nil || assetMaxBytes <= 0 {
		return nil, ErrImportUnavailable
	}
	return &PostgresImportRepository{pool: pool, storage: storage, assetMaxBytes: assetMaxBytes}, nil
}

func (r *PostgresImportRepository) Import(ctx context.Context, plan importPlan) (ImportResult, error) {
	if r == nil || r.pool == nil || r.storage == nil || !validImportPlan(plan) {
		return ImportResult{}, ErrImportUnavailable
	}
	if record, err := coursessqlc.New(r.pool).GetImportRecordByFingerprint(ctx, plan.fingerprint); err == nil {
		return r.replay(ctx, record, plan)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ImportResult{}, ErrImportPersistenceFailed
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ImportResult{}, ErrImportPersistenceFailed
	}
	committed := false
	createdAssets := make([]assets.Asset, 0, len(plan.assets))
	var assetService *assets.ImportIngestionService
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	fail := func(cause error) (ImportResult, error) {
		_ = tx.Rollback(ctx)
		cleanupErr := compensateImportedAssets(assetService, createdAssets)
		if cleanupErr != nil {
			return ImportResult{}, errors.Join(cause, ErrImportCleanupFailed)
		}
		return ImportResult{}, cause
	}

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, plan.manifest.Course.OriginCourseID); err != nil {
		return fail(ErrImportPersistenceFailed)
	}
	q := coursessqlc.New(tx)
	if record, err := q.GetImportRecordByFingerprint(ctx, plan.fingerprint); err == nil {
		if err := tx.Rollback(ctx); err != nil {
			return ImportResult{}, ErrImportPersistenceFailed
		}
		return r.replay(ctx, record, plan)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrImportPersistenceFailed)
	}
	if record, err := q.GetImportRecordByPortableSourceVersion(ctx, coursessqlc.GetImportRecordByPortableSourceVersionParams{PortableSourceCourseID: plan.manifest.Course.OriginCourseID, PortableSourceCourseVersionID: plan.manifest.Course.OriginCourseVersionID}); err == nil {
		if record.PackageFingerprint == plan.fingerprint {
			if err := tx.Rollback(ctx); err != nil {
				return ImportResult{}, ErrImportPersistenceFailed
			}
			return r.replay(ctx, record, plan)
		}
		return fail(ErrImportSourceVersionConflict)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrImportPersistenceFailed)
	}

	localCourseID, err := r.resolveCourse(ctx, q, tx, plan)
	if err != nil {
		return fail(err)
	}
	record, err := q.CreateImportRecord(ctx, coursessqlc.CreateImportRecordParams{
		PackageFingerprint: plan.fingerprint, PortableSourceCourseID: plan.manifest.Course.OriginCourseID,
		PortableSourceCourseVersionID: plan.manifest.Course.OriginCourseVersionID, SourceVersion: plan.course.Version,
		ImportedAt: pgtype.Timestamptz{Time: plan.importedAt, Valid: true},
	})
	if err != nil {
		return fail(mapImportWriteError(err))
	}

	assetService, err = assets.NewImportIngestionService(assetspostgres.New(tx), r.storage, r.assetMaxBytes)
	if err != nil {
		return fail(ErrImportUnavailable)
	}
	bindings := make([]courses.PublishedAssetBinding, 0, len(plan.assets))
	for _, packaged := range plan.assets {
		digest, parseErr := assets.ParseSHA256Digest(packaged.entry.SHA256)
		if parseErr != nil {
			return fail(ErrImportStorageFailed)
		}
		stored, ingestErr := assetService.ImportValidatedAsset(ctx, assets.ImportedAssetInput{
			ImportID: record.ID.String(), PackageAssetKey: packaged.entry.AssetKey, OriginalFilename: packaged.entry.OriginalFilename,
			MediaType: packaged.entry.MediaType, ByteSize: packaged.entry.ByteSize, SHA256Digest: digest,
			Content: bytes.NewReader(packaged.bytes),
		})
		if ingestErr != nil {
			return fail(ErrImportStorageFailed)
		}
		createdAssets = append(createdAssets, stored)
		bindings = append(bindings, courses.PublishedAssetBinding{AssetKey: packaged.entry.AssetKey, StorageObjectID: string(stored.StorageObjectID), OriginalFilename: stored.OriginalFilename, MediaType: stored.MediaType, ByteSize: stored.ByteSize, SHA256Digest: string(stored.SHA256Digest)})
	}

	immutable, err := immutableFromPlan(plan, localCourseID, bindings, record.ID.String())
	if err != nil {
		return fail(ErrImportPersistenceFailed)
	}
	courseRepo := coursespostgres.New(r.pool)
	stored, err := courseRepo.StoreImmutableCourseVersionInTx(ctx, tx, immutable)
	if err != nil {
		return fail(mapImportWriteError(err))
	}
	translationRepo := translationspostgres.New(r.pool)
	translationLanguages := make([]string, 0, len(plan.translations))
	translationProvenance := translations.ImportedPublicationProvenance{ImportID: record.ID.String(), PackageFingerprint: plan.fingerprint, PortableSourceCourseID: plan.manifest.Course.OriginCourseID, PortableSourceCourseVersionID: plan.manifest.Course.OriginCourseVersionID, ImportedAt: plan.importedAt}
	for _, translated := range plan.translations {
		language, parseErr := courses.NormalizeLanguageTag(translated.Language)
		if parseErr != nil {
			return fail(ErrImportPersistenceFailed)
		}
		if _, err := translationRepo.StoreImportedPublicationInTx(ctx, tx, stored, language, translated.Tree, translationProvenance, translated.PublishedAt); err != nil {
			return fail(mapImportWriteError(err))
		}
		translationLanguages = append(translationLanguages, translated.Language)
	}
	sort.Strings(translationLanguages)
	if _, err := q.FinalizeImportRecord(ctx, coursessqlc.FinalizeImportRecordParams{ID: record.ID, TargetCourseID: uuidValue(string(localCourseID)), TargetCourseVersionID: uuidValue(string(stored.ID))}); err != nil {
		return fail(mapImportWriteError(err))
	}
	if err := tx.Commit(ctx); err != nil {
		return fail(ErrImportPersistenceFailed)
	}
	committed = true
	return ImportResult{ImportID: record.ID.String(), CourseID: localCourseID, CourseVersionID: stored.ID, Version: stored.CourseVersion.Version, Disposition: ImportCreated, TranslationLanguages: translationLanguages}, nil
}

func (r *PostgresImportRepository) resolveCourse(ctx context.Context, q *coursessqlc.Queries, tx pgx.Tx, plan importPlan) (courses.CourseID, error) {
	mapping, err := q.GetPortableSourceCourseMapping(ctx, plan.manifest.Course.OriginCourseID)
	if err == nil {
		return courses.CourseID(mapping.String()), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", ErrImportPersistenceFailed
	}
	created, err := coursespostgres.New(tx).CreateCourse(ctx, "import-"+plan.fingerprint)
	if err != nil {
		return "", mapImportWriteError(err)
	}
	if _, err := q.CreatePortableSourceCourseMapping(ctx, coursessqlc.CreatePortableSourceCourseMappingParams{PortableSourceCourseID: plan.manifest.Course.OriginCourseID, LocalCourseID: uuidValue(string(created.ID))}); err != nil {
		return "", mapImportWriteError(err)
	}
	return created.ID, nil
}

func (r *PostgresImportRepository) replay(ctx context.Context, record coursessqlc.PortabilityImportRecord, plan importPlan) (ImportResult, error) {
	if record.PackageFingerprint != plan.fingerprint || record.PortableSourceCourseID != plan.manifest.Course.OriginCourseID || record.PortableSourceCourseVersionID != plan.manifest.Course.OriginCourseVersionID || record.SourceVersion != plan.course.Version || !record.TargetCourseID.Valid || !record.TargetCourseVersionID.Valid {
		return ImportResult{}, ErrImportPersistenceFailed
	}
	version, err := courses.ParseVersion(record.SourceVersion)
	if err != nil {
		return ImportResult{}, ErrImportPersistenceFailed
	}
	courseID := courses.CourseID(record.TargetCourseID.String())
	versionID := courses.CourseVersionID(record.TargetCourseVersionID.String())
	stored, err := coursespostgres.New(r.pool).GetImmutableCourseVersion(ctx, versionID)
	if err != nil || stored.CourseVersion.CourseID != courseID || stored.CourseVersion.Version != version {
		return ImportResult{}, ErrImportPersistenceFailed
	}
	languages, err := translationspostgres.New(r.pool).ListLanguages(ctx, versionID)
	if err != nil {
		return ImportResult{}, ErrImportPersistenceFailed
	}
	resultLanguages := make([]string, len(languages))
	for index, language := range languages {
		resultLanguages[index] = string(language)
	}
	return ImportResult{ImportID: record.ID.String(), CourseID: courseID, CourseVersionID: versionID, Version: version, Disposition: ImportReplayed, TranslationLanguages: resultLanguages}, nil
}

func compensateImportedAssets(service *assets.ImportIngestionService, created []assets.Asset) error {
	if len(created) == 0 {
		return nil
	}
	if service == nil {
		return assets.ErrRollbackIncomplete
	}
	var cleanup error
	for index := len(created) - 1; index >= 0; index-- {
		if err := service.CompensateImportedAsset(context.Background(), created[index]); err != nil {
			cleanup = errors.Join(cleanup, err)
		}
	}
	return cleanup
}

func immutableFromPlan(plan importPlan, courseID courses.CourseID, bindings []courses.PublishedAssetBinding, importID string) (courses.ImmutableCourseVersion, error) {
	version, err := courses.ParseVersion(plan.course.Version)
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	license := courses.ContentLicense{Kind: courses.ContentLicenseKind(plan.manifest.Course.License.Kind), Identifier: plan.manifest.Course.License.Identifier, DisplayName: plan.manifest.Course.License.DisplayName, URL: plan.manifest.Course.License.URL, CustomText: plan.manifest.Course.License.CustomText}
	attribution := make([]courses.ContributorSnapshot, 0, len(plan.manifest.Course.Attribution))
	for _, contributor := range plan.manifest.Course.Attribution {
		attribution = append(attribution, courses.ContributorSnapshot{DisplayName: contributor.DisplayName, Role: courses.ContributorRole(contributor.Role), Order: contributor.Order})
	}
	modules := make([]courses.ImmutableCourseVersionModule, 0, len(plan.course.Modules))
	for _, module := range plan.course.Modules {
		localModule := courses.ImmutableCourseVersionModule{SourceID: uuid.NewString(), StableKey: module.StableKey, Title: module.Title, Description: module.Description, Position: module.Position, Lessons: make([]courses.ImmutableCourseVersionLesson, 0, len(module.Lessons))}
		for _, lesson := range module.Lessons {
			localModule.Lessons = append(localModule.Lessons, courses.ImmutableCourseVersionLesson{SourceID: uuid.NewString(), StableKey: lesson.StableKey, Title: lesson.Title, Description: lesson.Description, LearningObjectives: append([]string{}, lesson.LearningObjectives...), EstimatedDurationMinutes: lesson.EstimatedDurationMinutes, Position: lesson.Position, PrerequisiteStableKeys: append([]string{}, lesson.PrerequisiteStableKeys...), Content: lesson.Content})
		}
		modules = append(modules, localModule)
	}
	assessments := make([]courses.PublishedAssessmentBinding, 0, len(plan.assessments.Assessments))
	for _, assessment := range plan.assessments.Assessments {
		binding := courses.PublishedAssessmentBinding{AssessmentKey: assessment.AssessmentKey}
		for _, question := range assessment.Questions {
			published := courses.PublishedAssessmentQuestion{StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position, CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...)}
			for _, option := range question.Options {
				published.Options = append(published.Options, courses.PublishedAssessmentOption(option))
			}
			for _, item := range question.LeftItems {
				published.LeftItems = append(published.LeftItems, courses.PublishedAssessmentItem(item))
			}
			for _, item := range question.RightItems {
				published.RightItems = append(published.RightItems, courses.PublishedAssessmentItem(item))
			}
			for _, pair := range question.CorrectPairs {
				published.CorrectPairs = append(published.CorrectPairs, courses.PublishedAssessmentPair(pair))
			}
			binding.Questions = append(binding.Questions, published)
		}
		assessments = append(assessments, binding)
	}
	return courses.ImmutableCourseVersion{CourseVersion: courses.CourseVersionInput{CourseID: courseID, Version: version, Status: courses.CourseVersionPublished, Title: plan.course.Title, Description: plan.course.Description, LearningObjectives: append([]string(nil), plan.course.LearningObjectives...), SourceLanguage: courses.LanguageTag(plan.course.Language), Changelog: plan.course.Changelog, License: license, Attribution: attribution, PublishedAt: plan.importedAt}, Publication: courses.PublicationProvenance{Origin: courses.PublicationOriginImported, Imported: &courses.ImportedPublicationProvenance{ImportID: importID, PackageFingerprint: plan.fingerprint, PortableSourceCourseID: plan.manifest.Course.OriginCourseID, PortableSourceCourseVersionID: plan.manifest.Course.OriginCourseVersionID, SourceVersion: version, ImportedAt: plan.importedAt}}, Modules: modules, AssetBindings: bindings, AssessmentBindings: assessments}, nil
}

func validImportPlan(plan importPlan) bool {
	return plan.fingerprint != "" && plan.importedAt.Equal(plan.importedAt.UTC()) && !plan.importedAt.IsZero() && plan.manifest.Format == Format && plan.manifest.FormatVersion == FormatVersion && plan.manifest.Course.OriginCourseID != "" && plan.manifest.Course.OriginCourseVersionID != "" && plan.course.Version != ""
}

func uuidValue(value string) pgtype.UUID { var id pgtype.UUID; _ = id.Scan(value); return id }

func mapImportWriteError(err error) error {
	if errors.Is(err, courses.ErrCourseVersionAlreadyExists) {
		return ErrImportLocalCourseVersionConflict
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrImportSourceVersionConflict
	}
	return ErrImportPersistenceFailed
}
