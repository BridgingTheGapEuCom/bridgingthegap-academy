package authoring

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// PublishedReviewPublication is the safe, read-side association between one
// exact Review and its immutable Courses artifact. It deliberately excludes
// internal publisher and Review/Draft provenance identifiers.
type PublishedReviewPublication struct {
	CourseID      courses.CourseID
	CourseVersion courses.Version
	PublishedAt   time.Time
}

// ReviewPublicationStatus is an exact-Review projection for Authoring UI
// reads. CanPublish comes from the Authoring capability policy; Publishable
// and Issues come from the frozen Review snapshot validator.
type ReviewPublicationStatus struct {
	CanPublish  bool
	Publishable bool
	Issues      []PublicationValidationIssue
	Published   *PublishedReviewPublication
}

// ReviewPublicationStatusService composes existing Authoring boundaries for a
// private read model. It neither reads mutable Draft content nor writes either
// Authoring or Courses state.
type ReviewPublicationStatusService struct {
	publications PublicationRecordRepository
	authorizer   Authorizer
	validator    PublicationValidator
}

func NewReviewPublicationStatusService(publications PublicationRecordRepository, authorizer Authorizer) *ReviewPublicationStatusService {
	return &ReviewPublicationStatusService{
		publications: publications,
		authorizer:   authorizer,
		validator:    NewPublicationValidator(),
	}
}

// Status projects publication readiness and any exact publication fact for a
// Review that has already been loaded through the normal Draft-scoped Review
// read boundary. Authorization denial is intentionally represented as a false
// display capability; an unavailable authorization dependency remains an
// operational error.
func (s *ReviewPublicationStatusService) Status(ctx context.Context, actor identity.AuthenticatedActor, cycle ReviewCycle, snapshot ReviewSnapshot) (ReviewPublicationStatus, error) {
	if s == nil || s.publications == nil || s.authorizer == nil || cycle.ID == "" || cycle.DraftID == "" {
		return ReviewPublicationStatus{}, ErrPublicationInvalidInput
	}

	validation := s.validator.Validate(cycle, &snapshot)
	status := ReviewPublicationStatus{
		Publishable: validation.Publishable,
		Issues:      validation.Issues,
	}
	if err := s.authorizer.Authorize(ctx, actor, CapabilityPublish, DraftResource(cycle.DraftID)); err == nil {
		status.CanPublish = true
	} else if !errors.Is(err, ErrAuthorizationDenied) {
		return ReviewPublicationStatus{}, err
	}

	record, err := s.publications.GetPublication(ctx, cycle.ID)
	if errors.Is(err, ErrNotFound) {
		return status, nil
	}
	if err != nil {
		return ReviewPublicationStatus{}, err
	}
	if !publicationRecordMatchesReview(record, cycle) {
		return ReviewPublicationStatus{}, ErrPublicationConflict
	}
	status.Published = &PublishedReviewPublication{
		CourseID:      record.CourseID,
		CourseVersion: record.CourseVersion,
		PublishedAt:   record.PublishedAt,
	}
	return status, nil
}

func publicationRecordMatchesReview(record PublicationRecord, cycle ReviewCycle) bool {
	if record.ReviewID != cycle.ID || record.ReviewRevision != cycle.Revision ||
		record.DraftID != cycle.DraftID || record.DraftRevision != cycle.DraftRevision ||
		record.CourseID == "" || record.CourseVersionID == "" || record.PublishedAt.IsZero() {
		return false
	}
	_, err := courses.ParseVersion(record.CourseVersion.String())
	return err == nil
}
