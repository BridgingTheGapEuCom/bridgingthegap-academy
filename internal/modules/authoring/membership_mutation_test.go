package authoring

import (
	"context"
	"errors"
	"testing"
)

type membershipMutationRepositoryFake struct {
	draft   CourseDraft
	members map[string]WorkspaceMember
}

func (r *membershipMutationRepositoryFake) AddMemberForDraft(_ context.Context, draftID DraftID, expected int64, userID string, role MemberRole) (WorkspaceMember, CourseDraft, error) {
	if draftID != r.draft.ID || expected != r.draft.Revision {
		return WorkspaceMember{}, CourseDraft{}, ErrRevisionMismatch
	}
	if _, found := r.members[userID]; found {
		return WorkspaceMember{}, CourseDraft{}, ErrConflict
	}
	member := WorkspaceMember{ID: "member-" + userID, WorkspaceID: "workspace", UserID: userID, Role: role}
	r.members[userID] = member
	r.draft.Revision++
	return member, r.draft, nil
}

func (r *membershipMutationRepositoryFake) ChangeMemberRoleForDraft(_ context.Context, draftID DraftID, expected int64, userID string, role MemberRole) (WorkspaceMember, CourseDraft, error) {
	member, found := r.members[userID]
	if !found || draftID != r.draft.ID {
		return WorkspaceMember{}, CourseDraft{}, ErrNotFound
	}
	if expected != r.draft.Revision {
		return WorkspaceMember{}, CourseDraft{}, ErrRevisionMismatch
	}
	if member.Role == role {
		return WorkspaceMember{}, CourseDraft{}, ErrInvalidStructure
	}
	member.Role = role
	r.members[userID] = member
	r.draft.Revision++
	return member, r.draft, nil
}

func (r *membershipMutationRepositoryFake) RevokeMemberForDraft(_ context.Context, draftID DraftID, expected int64, userID string) (WorkspaceMember, CourseDraft, error) {
	member, found := r.members[userID]
	if !found || draftID != r.draft.ID {
		return WorkspaceMember{}, CourseDraft{}, ErrNotFound
	}
	if expected != r.draft.Revision {
		return WorkspaceMember{}, CourseDraft{}, ErrRevisionMismatch
	}
	delete(r.members, userID)
	r.draft.Revision++
	return member, r.draft, nil
}

func TestMembershipMutationServiceUsesScopedCapabilityAndDraftRevision(t *testing.T) {
	draft := mutationDraft()
	repository := &membershipMutationRepositoryFake{draft: draft, members: map[string]WorkspaceMember{}}
	authorizer := &draftMutationAuthorizerFake{}
	service := NewMembershipMutationService(repository, authorizer)

	added, err := service.AddMember(context.Background(), resolvedActor(t), draft.ID, 3, "member", MemberAuthor)
	if err != nil || added.Member.Role != MemberAuthor || added.Draft.Revision != 4 || authorizer.capability != CapabilityMembersManage || authorizer.resource != DraftResource(draft.ID) {
		t.Fatalf("member add did not use scoped membership capability and draft revision: %#v err=%v", added, err)
	}
	promoted, err := service.ChangeRole(context.Background(), resolvedActor(t), draft.ID, 4, "member", MemberMaintainer)
	if err != nil || promoted.Member.Role != MemberMaintainer || promoted.Draft.Revision != 5 {
		t.Fatalf("member role change did not return committed state: %#v err=%v", promoted, err)
	}
	if _, err := service.RevokeMember(context.Background(), resolvedActor(t), draft.ID, 4, "member"); !errors.Is(err, ErrRevisionMismatch) {
		t.Fatalf("stale membership mutation = %v", err)
	}
	authorizer.err = ErrAuthorizationDenied
	if _, err := service.RevokeMember(context.Background(), resolvedActor(t), draft.ID, 5, "member"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("denied membership mutation did not hide draft: %v", err)
	}
}
