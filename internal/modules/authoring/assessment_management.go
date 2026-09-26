package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

const DefaultAssessmentListLimit = 20
const MaxAssessmentListLimit = 100

// AssessmentDefinition is mutable Authoring content. It intentionally uses
// neutral Assessment values; the HTTP layer owns the explicit answer-bearing
// Authoring DTO and must never reuse that DTO for learners.
type AssessmentDefinition struct {
	Title     string
	Questions []assessments.Question
}

type AssessmentListPage struct {
	Items                []assessments.AssessmentSummary
	Limit, Offset, Total int
}

// AssessmentManagementService authorizes an exact Draft before every neutral
// Assessment operation. The Assessment module remains unaware of membership
// policy and Authoring transport concerns.
type AssessmentManagementService struct {
	repository assessments.Repository
	authorizer Authorizer
}

func NewAssessmentManagementService(repository assessments.Repository, authorizer Authorizer) *AssessmentManagementService {
	return &AssessmentManagementService{repository: repository, authorizer: authorizer}
}

func (s *AssessmentManagementService) Create(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, definition AssessmentDefinition) (assessments.Assessment, error) {
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return assessments.Assessment{}, err
	}
	return s.repository.CreateAssessment(ctx, assessments.AssessmentInput{
		OwnerDraftID: string(draftID), CreatedByUserID: string(actor.UserID()), Title: definition.Title, Questions: definition.Questions,
	})
}

func (s *AssessmentManagementService) List(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, limit, offset int) (AssessmentListPage, error) {
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return AssessmentListPage{}, err
	}
	if limit == 0 {
		limit = DefaultAssessmentListLimit
	}
	if limit < 1 || limit > MaxAssessmentListLimit || offset < 0 {
		return AssessmentListPage{}, assessments.ErrInvalidAssessment
	}
	items, total, err := s.repository.ListAssessmentSummariesForDraft(ctx, string(draftID), limit, offset)
	if err != nil {
		return AssessmentListPage{}, err
	}
	return AssessmentListPage{Items: items, Limit: limit, Offset: offset, Total: total}, nil
}

func (s *AssessmentManagementService) Get(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, id assessments.AssessmentID) (assessments.Assessment, error) {
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return assessments.Assessment{}, err
	}
	assessment, err := s.repository.GetAssessment(ctx, id)
	if err != nil {
		return assessments.Assessment{}, err
	}
	if assessment.OwnerDraftID != string(draftID) {
		return assessments.Assessment{}, ErrNotFound
	}
	return assessment, nil
}

func (s *AssessmentManagementService) Update(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, id assessments.AssessmentID, expectedRevision int64, definition AssessmentDefinition) (assessments.Assessment, error) {
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return assessments.Assessment{}, err
	}
	assessment, err := s.repository.GetAssessment(ctx, id)
	if err != nil {
		return assessments.Assessment{}, err
	}
	if assessment.OwnerDraftID != string(draftID) {
		return assessments.Assessment{}, ErrNotFound
	}
	return s.repository.UpdateAssessment(ctx, id, expectedRevision, assessments.AssessmentUpdate{Title: definition.Title, Questions: definition.Questions})
}

func (s *AssessmentManagementService) authorize(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID) error {
	if s == nil || s.repository == nil || s.authorizer == nil || draftID == "" {
		return ErrAuthorizationUnavailable
	}
	if err := s.authorizer.Authorize(ctx, actor, CapabilityAssessmentEdit, DraftResource(draftID)); err != nil {
		if errors.Is(err, ErrAuthorizationDenied) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
