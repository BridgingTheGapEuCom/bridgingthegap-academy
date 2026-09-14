package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// DraftMetadataRepository is the focused mutable Authoring contract used by
// metadata updates. It cannot write structure, lessons, membership, or any
// published Courses data.
type DraftMetadataRepository interface {
	GetDraft(context.Context, DraftID) (CourseDraft, error)
	UpdateDraftMetadata(context.Context, DraftID, int64, DraftMetadata) (CourseDraft, error)
}

// DraftMutationService applies a validated partial draft metadata update after
// a current, resource-scoped capability decision. A denied resource is hidden
// as ErrNotFound for private HTTP serving.
type DraftMutationService struct {
	repository DraftMetadataRepository
	authorizer Authorizer
}

func NewDraftMutationService(repository DraftMetadataRepository, authorizer Authorizer) *DraftMutationService {
	return &DraftMutationService{repository: repository, authorizer: authorizer}
}

func (s *DraftMutationService) UpdateDraft(ctx context.Context, actor identity.AuthenticatedActor, id DraftID, expectedRevision int64, patch DraftMetadataPatch) (CourseDraft, error) {
	if expectedRevision < 1 || patch.Empty() {
		return CourseDraft{}, ErrInvalidPatch
	}
	if s.repository == nil || s.authorizer == nil {
		return CourseDraft{}, ErrAuthorizationUnavailable
	}
	err := s.authorizer.Authorize(ctx, actor, CapabilityDraftEdit, DraftResource(id))
	if errors.Is(err, ErrAuthorizationDenied) {
		return CourseDraft{}, ErrNotFound
	}
	if err != nil {
		return CourseDraft{}, err
	}
	current, err := s.repository.GetDraft(ctx, id)
	if err != nil {
		return CourseDraft{}, err
	}
	metadata, err := patch.Apply(current.Metadata)
	if err != nil {
		return CourseDraft{}, ErrInvalidPatch
	}
	return s.repository.UpdateDraftMetadata(ctx, id, expectedRevision, metadata)
}
