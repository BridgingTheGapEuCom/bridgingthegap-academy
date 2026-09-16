package authoring

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// ReviewPublisher is the already-composed M4.5d publication boundary. The
// authenticated application service supplies actor and clock metadata; HTTP
// never supplies either value.
type ReviewPublisher interface {
	Publish(context.Context, PublishReviewCommand) (PublicationResult, error)
}

// PublicationApplicationService authorizes one exact Draft resource before
// invoking publication orchestration. It contains no validation, conversion,
// persistence, or reconciliation rules of its own.
type PublicationApplicationService struct {
	publisher  ReviewPublisher
	authorizer Authorizer
	now        func() time.Time
}

func NewPublicationApplicationService(publisher ReviewPublisher, authorizer Authorizer, now func() time.Time) *PublicationApplicationService {
	return &PublicationApplicationService{publisher: publisher, authorizer: authorizer, now: now}
}

func (s *PublicationApplicationService) Publish(ctx context.Context, actor identity.AuthenticatedActor, draft DraftID, review ReviewID, expected int64) (PublicationResult, error) {
	if s == nil || s.publisher == nil || s.authorizer == nil || s.now == nil || draft == "" || review == "" || expected < 1 {
		return PublicationResult{}, ErrPublicationInvalidInput
	}
	if err := s.authorizer.Authorize(ctx, actor, CapabilityPublish, DraftResource(draft)); err != nil {
		if errors.Is(err, ErrAuthorizationDenied) {
			return PublicationResult{}, ErrNotFound
		}
		return PublicationResult{}, err
	}
	publishedAt := s.now().UTC()
	if publishedAt.IsZero() {
		return PublicationResult{}, ErrPublicationInvalidInput
	}
	return s.publisher.Publish(ctx, PublishReviewCommand{
		DraftID: draft, ReviewID: review, ExpectedReviewRevision: expected,
		PublishedByUserID: string(actor.UserID()), PublishedAt: publishedAt,
	})
}
