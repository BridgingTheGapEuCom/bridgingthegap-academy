package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// ReviewApplicationService is the resource-scoped boundary used by transports.
// Authorization and per-cycle decision policy remain separate checks.
type ReviewApplicationService struct {
	repository     ReviewRepository
	authorizer     Authorizer
	decisionPolicy ReviewDecisionPolicy
}

func NewReviewApplicationService(repository ReviewRepository, authorizer Authorizer) *ReviewApplicationService {
	return NewReviewApplicationServiceWithDecisionPolicy(repository, authorizer, NewReviewDecisionPolicy(true))
}

func NewReviewApplicationServiceWithDecisionPolicy(repository ReviewRepository, authorizer Authorizer, decisionPolicy ReviewDecisionPolicy) *ReviewApplicationService {
	if decisionPolicy == nil {
		decisionPolicy = NewReviewDecisionPolicy(true)
	}
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
	cycle, _, err := s.repository.GetReviewForDraft(ctx, draft, review)
	if err != nil {
		return ReviewCycle{}, err
	}
	// Mutable state is checked before policy, so a stale or terminal request
	// does not reveal policy outcome. Persistence repeats these checks under its
	// Review lock; submitter provenance is immutable, so policy cannot race.
	if err := cycle.CanDecide(expected); err != nil {
		return ReviewCycle{}, err
	}
	actorID := string(actor.UserID())
	if err := s.decisionPolicy.Check(ReviewDecisionPolicyInput{
		DecisionActorUserID:     actorID,
		ReviewSubmittedByUserID: cycle.SubmittedByUserID,
	}); err != nil {
		return ReviewCycle{}, err
	}
	return s.repository.DecideReviewForDraft(ctx, draft, review, expected, decision, actorID, message)
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
