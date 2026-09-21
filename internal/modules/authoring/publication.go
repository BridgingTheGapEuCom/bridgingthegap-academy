package authoring

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

var (
	ErrPublicationInvalidInput       = errors.New("invalid publication input")
	ErrPublicationConflict           = errors.New("publication record conflict")
	ErrPublicationProvenanceMismatch = errors.New("publication provenance mismatch")
)

// PublicationRecord is the Authoring-owned fact that an exact Review cycle
// produced one Courses-owned immutable version. It does not alter Review state.
type PublicationRecord struct {
	ReviewID          ReviewID
	ReviewRevision    int64
	DraftID           DraftID
	DraftRevision     int64
	CourseID          courses.CourseID
	CourseVersion     courses.Version
	CourseVersionID   courses.CourseVersionID
	PublishedAt       time.Time
	PublishedByUserID string
	RecordedAt        time.Time
}

type PublicationRecordRepository interface {
	RecordPublication(context.Context, PublicationRecord) (PublicationRecord, error)
	GetPublication(context.Context, ReviewID) (PublicationRecord, error)
}

// PublicationCourseVersionStore is the Courses-owned application boundary
// required for persistence and replay reconciliation.
type PublicationCourseVersionStore interface {
	Store(context.Context, courses.ImmutableCourseVersion) (courses.ImmutableCourseVersion, error)
	GetByReviewID(context.Context, string) (courses.ImmutableCourseVersion, error)
	GetByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
}

type PublishReviewCommand struct {
	DraftID                DraftID
	ReviewID               ReviewID
	ExpectedReviewRevision int64
	PublishedByUserID      string
	PublishedAt            time.Time
	Attribution            []courses.ContributorSnapshot
}

type PublicationResult struct {
	ReviewID        ReviewID
	ReviewRevision  int64
	CourseID        courses.CourseID
	CourseVersion   courses.Version
	CourseVersionID courses.CourseVersionID
	PublishedAt     time.Time
	Reconciled      bool
}

// PublicationService coordinates module-owned boundaries without pretending
// their separate database transactions are one distributed transaction.
type PublicationService struct {
	reviews      ReviewRepository
	converter    PublicationConverter
	assets       PublicationAssetResolver
	versions     PublicationCourseVersionStore
	publications PublicationRecordRepository
}

func NewPublicationService(reviews ReviewRepository, versions PublicationCourseVersionStore, publications PublicationRecordRepository) *PublicationService {
	return &PublicationService{reviews: reviews, converter: NewPublicationConverter(), versions: versions, publications: publications}
}

func NewPublicationServiceWithAssets(reviews ReviewRepository, resolver PublicationAssetResolver, versions PublicationCourseVersionStore, publications PublicationRecordRepository) *PublicationService {
	return &PublicationService{reviews: reviews, converter: NewPublicationConverter(), assets: resolver, versions: versions, publications: publications}
}

func (s *PublicationService) Publish(ctx context.Context, command PublishReviewCommand) (PublicationResult, error) {
	if s == nil || s.reviews == nil || s.versions == nil || s.publications == nil || command.DraftID == "" || command.ReviewID == "" || command.ExpectedReviewRevision < 1 || ValidateReviewActor(command.PublishedByUserID) != nil || command.PublishedAt.IsZero() {
		return PublicationResult{}, ErrPublicationInvalidInput
	}

	cycle, snapshot, err := s.reviews.GetReviewForDraft(ctx, command.DraftID, command.ReviewID)
	if err != nil {
		return PublicationResult{}, err
	}
	if cycle.Revision != command.ExpectedReviewRevision {
		return PublicationResult{}, ErrReviewStale
	}
	if cycle.Status != ReviewApproved {
		return PublicationResult{}, ErrReviewInvalidState
	}
	if validation := validatePublicationBeforeAssetResolution(s.converter.validator, cycle, &snapshot); !validation.Publishable {
		return PublicationResult{}, &PublicationValidationFailure{Result: validation}
	}

	attribution := command.Attribution
	if attribution == nil {
		// The browser never supplies historical attribution. Keep the frozen
		// submitter as an internal account reference, while using a safe public
		// display label until the Review snapshot carries public attribution.
		attribution = []courses.ContributorSnapshot{{
			UserID: cycle.SubmittedByUserID, DisplayName: "Author",
			Role: courses.ContributorAuthor, Order: 0,
		}}
	}

	// Recovery checks the Courses-owned immutable aggregate before consulting
	// current Asset state. Once Courses committed, its frozen bindings are the
	// authority even if the separate Authoring publication-fact write failed.
	existing, existingErr := s.versions.GetByReviewID(ctx, string(cycle.ID))
	if existingErr == nil && existing.ID != "" {
		assessmentResolution := resolvePublicationAssessments(snapshot)
		if len(assessmentResolution.Issues) != 0 {
			return PublicationResult{}, ErrPublicationProvenanceMismatch
		}
		candidate, err := s.converter.Convert(cycle, &snapshot, PublicationConversionMetadata{
			PublishedAt: existing.CourseVersion.PublishedAt, PublishedByUserID: existing.Provenance.PublishedByUserID,
			Attribution: attribution, AssetBindings: existing.AssetBindings, AssessmentBindings: assessmentResolution.Bindings,
		})
		if err != nil || !samePublicationSource(existing, candidate) {
			return PublicationResult{}, ErrPublicationProvenanceMismatch
		}
		return s.recordResult(ctx, existing, true)
	}
	if existingErr != nil && !errors.Is(existingErr, courses.ErrNotFound) {
		return PublicationResult{}, existingErr
	}

	bindings := []courses.PublishedAssetBinding(nil)
	assetResolution := PublicationAssetResolution{Bindings: bindings, Issues: []PublicationValidationIssue{}}
	if len(publicationAssetUses(snapshot)) > 0 && s.assets != nil {
		resolution, err := s.assets.Resolve(ctx, cycle.DraftID, snapshot)
		if err != nil {
			return PublicationResult{}, err
		}
		assetResolution = resolution
		bindings = resolution.Bindings
	}
	assessmentResolution := resolvePublicationAssessments(snapshot)
	validation := validateResolvedPublication(s.converter.validator, cycle, &snapshot, assetResolution, assessmentResolution)
	if !validation.Publishable {
		return PublicationResult{}, &PublicationValidationFailure{Result: validation}
	}
	candidate, err := s.converter.Convert(cycle, &snapshot, PublicationConversionMetadata{
		PublishedAt: command.PublishedAt, PublishedByUserID: command.PublishedByUserID,
		Attribution: attribution, AssetBindings: bindings, AssessmentBindings: assessmentResolution.Bindings,
	})
	if err != nil {
		return PublicationResult{}, err
	}

	stored, err := s.versions.Store(ctx, candidate)
	reconciled := false
	if errors.Is(err, courses.ErrCourseVersionAlreadyExists) {
		stored, err = s.reconcileExisting(ctx, candidate)
		reconciled = err == nil
	}
	if err != nil {
		return PublicationResult{}, err
	}

	return s.recordResult(ctx, stored, reconciled)
}

func (s *PublicationService) recordResult(ctx context.Context, stored courses.ImmutableCourseVersion, reconciled bool) (PublicationResult, error) {
	record, err := s.publications.RecordPublication(ctx, publicationRecordFromVersion(stored))
	if err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{
		ReviewID: record.ReviewID, ReviewRevision: record.ReviewRevision,
		CourseID: record.CourseID, CourseVersion: record.CourseVersion,
		CourseVersionID: record.CourseVersionID, PublishedAt: record.PublishedAt,
		Reconciled: reconciled,
	}, nil
}

func (s *PublicationService) reconcileExisting(ctx context.Context, candidate courses.ImmutableCourseVersion) (courses.ImmutableCourseVersion, error) {
	existing, err := s.versions.GetByReviewID(ctx, candidate.Provenance.ReviewID)
	if err == nil {
		if !samePublicationSource(existing, candidate) {
			return courses.ImmutableCourseVersion{}, ErrPublicationProvenanceMismatch
		}
		return existing, nil
	}
	if !errors.Is(err, courses.ErrNotFound) {
		return courses.ImmutableCourseVersion{}, err
	}
	// A logical version owned by another Review is a real conflict. The read
	// also distinguishes that expected collision from a lookup outage.
	if _, lookupErr := s.versions.GetByCourseAndVersion(ctx, candidate.CourseVersion.CourseID, candidate.CourseVersion.Version); lookupErr != nil && !errors.Is(lookupErr, courses.ErrNotFound) {
		return courses.ImmutableCourseVersion{}, lookupErr
	}
	return courses.ImmutableCourseVersion{}, courses.ErrCourseVersionAlreadyExists
}

func publicationRecordFromVersion(version courses.ImmutableCourseVersion) PublicationRecord {
	return PublicationRecord{
		ReviewID: ReviewID(version.Provenance.ReviewID), ReviewRevision: version.Provenance.ReviewRevision,
		DraftID: DraftID(version.Provenance.DraftID), DraftRevision: version.Provenance.DraftRevision,
		CourseID: version.CourseVersion.CourseID, CourseVersion: version.CourseVersion.Version,
		CourseVersionID: version.ID, PublishedAt: version.CourseVersion.PublishedAt,
		PublishedByUserID: version.Provenance.PublishedByUserID,
	}
}

// samePublicationSource compares all frozen Review-derived data while treating
// persistence IDs and explicit publication-time metadata as authoritative from
// the already-stored Courses aggregate during recovery.
func samePublicationSource(stored, candidate courses.ImmutableCourseVersion) bool {
	normalized := stored
	normalized.CourseVersion.PublishedAt = normalized.CourseVersion.PublishedAt.UTC()
	normalized.Provenance.SubmittedAt = normalized.Provenance.SubmittedAt.UTC()
	if normalized.Provenance.ApprovedAt != nil {
		approvedAt := normalized.Provenance.ApprovedAt.UTC()
		normalized.Provenance.ApprovedAt = &approvedAt
	}
	normalized.ID = ""
	for moduleIndex := range normalized.Modules {
		normalized.Modules[moduleIndex].ID = ""
		for lessonIndex := range normalized.Modules[moduleIndex].Lessons {
			normalized.Modules[moduleIndex].Lessons[lessonIndex].ID = ""
		}
	}
	candidate.CourseVersion.PublishedAt = normalized.CourseVersion.PublishedAt
	candidate.Provenance.PublishedByUserID = normalized.Provenance.PublishedByUserID
	candidate.Provenance.SubmittedAt = candidate.Provenance.SubmittedAt.UTC()
	if candidate.Provenance.ApprovedAt != nil {
		approvedAt := candidate.Provenance.ApprovedAt.UTC()
		candidate.Provenance.ApprovedAt = &approvedAt
	}
	return reflect.DeepEqual(normalized, candidate)
}
