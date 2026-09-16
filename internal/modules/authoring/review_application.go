package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// ReviewApplicationService is the resource-scoped boundary used by future
// transports. Reviewer-independence policy can be inserted here before a
// decision without changing persistence or trusting transport claims.
type ReviewApplicationService struct {
	repository     ReviewRepository
	authorizer     Authorizer
	decisionPolicy ReviewDecisionPolicy
}

func NewReviewApplicationService(repository ReviewRepository, authorizer Authorizer) *ReviewApplicationService {
	return NewReviewApplicationServiceWithDecisionPolicy(repository, authorizer, nil)
}

// NewReviewApplicationServiceWithDecisionPolicy accepts deployment policy at
// the composition boundary. M4.3a intentionally does not invoke it during
// decisions; M4.3b will apply it after authorization and before persistence.
func NewReviewApplicationServiceWithDecisionPolicy(repository ReviewRepository, authorizer Authorizer, decisionPolicy ReviewDecisionPolicy) *ReviewApplicationService {
	return &ReviewApplicationService{repository: repository, authorizer: authorizer, decisionPolicy: decisionPolicy}
}

func (s *ReviewApplicationService) Submit(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID, expected int64) (ReviewCycle, ReviewSnapshot, error) {
	if expected < 1 {
		return ReviewCycle{}, ReviewSnapshot{}, ErrReviewSnapshot
	}
	if err := s.authorize(ctx, actor, draft, CapabilityReviewSubmit); err != nil {
		return ReviewCycle{}, ReviewSnapshot{}, err
	}
	return s.repository.SubmitReview(ctx, draft, expected, string(actor.UserID()))
}

func (s *ReviewApplicationService) Active(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID) (ReviewCycle, ReviewSnapshot, error) {
	if err := s.authorize(ctx, actor, draft, CapabilityReviewRead); err != nil {
		return ReviewCycle{}, ReviewSnapshot{}, err
	}
	return s.repository.ActiveReview(ctx, draft)
}

func (s *ReviewApplicationService) Latest(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID) (ReviewCycle, ReviewSnapshot, error) {
	if err := s.authorize(ctx, actor, draft, CapabilityReviewRead); err != nil {
		return ReviewCycle{}, ReviewSnapshot{}, err
	}
	return s.repository.LatestReview(ctx, draft)
}

func (s *ReviewApplicationService) History(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID) ([]ReviewCycle, error) {
	if err := s.authorize(ctx, actor, draft, CapabilityReviewRead); err != nil {
		return nil, err
	}
	return s.repository.ListReviewHistory(ctx, draft)
}

func (s *ReviewApplicationService) Review(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID, review ReviewID) (ReviewCycle, ReviewSnapshot, error) {
	if err := s.authorize(ctx, actor, draft, CapabilityReviewRead); err != nil {
		return ReviewCycle{}, ReviewSnapshot{}, err
	}
	return s.repository.GetReviewForDraft(ctx, draft, review)
}

func (s *ReviewApplicationService) Approve(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID, review ReviewID, expected int64, message string) (ReviewCycle, error) {
	return s.decide(ctx, actor, draft, review, expected, ReviewApproved, message)
}

func (s *ReviewApplicationService) RequestChanges(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID, review ReviewID, expected int64, message string) (ReviewCycle, error) {
	return s.decide(ctx, actor, draft, review, expected, ReviewChangesRequested, message)
}

func (s *ReviewApplicationService) decide(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID, review ReviewID, expected int64, decision ReviewStatus, message string) (ReviewCycle, error) {
	if review == "" || expected < 1 || ValidateReviewMessage(message) != nil {
		return ReviewCycle{}, ErrReviewInvalidState
	}
	if err := s.authorize(ctx, actor, draft, CapabilityReviewDecide); err != nil {
		return ReviewCycle{}, err
	}
	return s.repository.DecideReviewForDraft(ctx, draft, review, expected, decision, string(actor.UserID()), message)
}

func (s *ReviewApplicationService) authorize(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID, capability Capability) error {
	if s == nil || s.repository == nil || s.authorizer == nil {
		return ErrAuthorizationUnavailable
	}
	err := s.authorizer.Authorize(ctx, actor, capability, DraftResource(draft))
	if errors.Is(err, ErrAuthorizationDenied) {
		return ErrNotFound
	}
	return err
}
