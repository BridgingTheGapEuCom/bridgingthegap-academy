package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// MembershipMutationRepository is the focused mutable contract for active
// workspace membership. It owns the last-maintainer invariant transactionally.
type MembershipMutationRepository interface {
	AddMemberForDraft(context.Context, DraftID, int64, string, MemberRole) (WorkspaceMember, CourseDraft, error)
	ChangeMemberRoleForDraft(context.Context, DraftID, int64, string, MemberRole) (WorkspaceMember, CourseDraft, error)
	RevokeMemberForDraft(context.Context, DraftID, int64, string) (WorkspaceMember, CourseDraft, error)
}

type MembershipMutationResult struct {
	Member WorkspaceMember
	Draft  CourseDraft
}

type MembershipMutationService struct {
	repository MembershipMutationRepository
	authorizer Authorizer
}

func NewMembershipMutationService(repository MembershipMutationRepository, authorizer Authorizer) *MembershipMutationService {
	return &MembershipMutationService{repository: repository, authorizer: authorizer}
}

func (s *MembershipMutationService) AddMember(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, expectedDraftRevision int64, userID string, role MemberRole) (MembershipMutationResult, error) {
	if expectedDraftRevision < 1 || !role.Valid() || userID == "" {
		return MembershipMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return MembershipMutationResult{}, err
	}
	member, draft, err := s.repository.AddMemberForDraft(ctx, draftID, expectedDraftRevision, userID, role)
	return MembershipMutationResult{Member: member, Draft: draft}, err
}

func (s *MembershipMutationService) ChangeRole(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, expectedDraftRevision int64, userID string, role MemberRole) (MembershipMutationResult, error) {
	if expectedDraftRevision < 1 || !role.Valid() || userID == "" {
		return MembershipMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return MembershipMutationResult{}, err
	}
	member, draft, err := s.repository.ChangeMemberRoleForDraft(ctx, draftID, expectedDraftRevision, userID, role)
	return MembershipMutationResult{Member: member, Draft: draft}, err
}

func (s *MembershipMutationService) RevokeMember(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, expectedDraftRevision int64, userID string) (MembershipMutationResult, error) {
	if expectedDraftRevision < 1 || userID == "" {
		return MembershipMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return MembershipMutationResult{}, err
	}
	member, draft, err := s.repository.RevokeMemberForDraft(ctx, draftID, expectedDraftRevision, userID)
	return MembershipMutationResult{Member: member, Draft: draft}, err
}

func (s *MembershipMutationService) authorize(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID) error {
	if s.repository == nil || s.authorizer == nil {
		return ErrAuthorizationUnavailable
	}
	err := s.authorizer.Authorize(ctx, actor, CapabilityMembersManage, DraftResource(draftID))
	if errors.Is(err, ErrAuthorizationDenied) {
		return ErrNotFound
	}
	return err
}
