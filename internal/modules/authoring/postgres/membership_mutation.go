package postgres

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/db/sqlc"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) AddMemberForDraft(ctx context.Context, draftID authoring.DraftID, expected int64, userID string, role authoring.MemberRole) (authoring.WorkspaceMember, authoring.CourseDraft, error) {
	if expected < 1 || !role.Valid() {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	return r.mutateMember(ctx, draftID, expected, userID, role, membershipAdd)
}

func (r *Repository) ChangeMemberRoleForDraft(ctx context.Context, draftID authoring.DraftID, expected int64, userID string, role authoring.MemberRole) (authoring.WorkspaceMember, authoring.CourseDraft, error) {
	if expected < 1 || !role.Valid() {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	return r.mutateMember(ctx, draftID, expected, userID, role, membershipChangeRole)
}

func (r *Repository) RevokeMemberForDraft(ctx context.Context, draftID authoring.DraftID, expected int64, userID string) (authoring.WorkspaceMember, authoring.CourseDraft, error) {
	if expected < 1 {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	return r.mutateMember(ctx, draftID, expected, userID, "", membershipRevoke)
}

type membershipMutation int

const (
	membershipAdd membershipMutation = iota
	membershipChangeRole
	membershipRevoke
)

func (r *Repository) mutateMember(ctx context.Context, draftID authoring.DraftID, expected int64, userID string, nextRole authoring.MemberRole, operation membershipMutation) (authoring.WorkspaceMember, authoring.CourseDraft, error) {
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, err
	}
	userKey, err := uuid(userID)
	if err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	workspace, err := q.GetWorkspace(ctx, draftKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(err)
	}
	var member sqlc.AuthoringWorkspaceMember
	switch operation {
	case membershipAdd:
		if err := r.lockActiveDraftRevision(ctx, tx, draftKey, expected); err != nil {
			return authoring.WorkspaceMember{}, authoring.CourseDraft{}, err
		}
		member, err = q.AddMember(ctx, sqlc.AddMemberParams{WorkspaceID: workspace.ID, UserID: userKey, Role: string(nextRole)})
	case membershipChangeRole, membershipRevoke:
		current, getErr := q.GetActiveMember(ctx, sqlc.GetActiveMemberParams{WorkspaceID: workspace.ID, UserID: userKey})
		if errors.Is(getErr, pgx.ErrNoRows) {
			return authoring.WorkspaceMember{}, authoring.CourseDraft{}, authoring.ErrNotFound
		}
		if getErr != nil {
			return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(getErr)
		}
		if err := r.lockActiveDraftRevision(ctx, tx, draftKey, expected); err != nil {
			return authoring.WorkspaceMember{}, authoring.CourseDraft{}, err
		}
		currentRole := authoring.MemberRole(current.Role)
		if currentRole == nextRole && operation == membershipChangeRole {
			return authoring.WorkspaceMember{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
		}
		if currentRole == authoring.MemberMaintainer && (operation == membershipRevoke || nextRole == authoring.MemberAuthor) {
			count, countErr := q.CountActiveMaintainers(ctx, workspace.ID)
			if countErr != nil {
				return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(countErr)
			}
			if count <= 1 {
				return authoring.WorkspaceMember{}, authoring.CourseDraft{}, authoring.ErrConflict
			}
		}
		member, err = q.RevokeMember(ctx, sqlc.RevokeMemberParams{WorkspaceID: workspace.ID, UserID: userKey})
		if err == nil && operation == membershipChangeRole {
			member, err = q.AddMember(ctx, sqlc.AddMemberParams{WorkspaceID: workspace.ID, UserID: userKey, Role: string(nextRole)})
		}
	}
	if err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: expected})
	if err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, storageError(err)
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.WorkspaceMember{}, authoring.CourseDraft{}, err
	}
	return mapMember(member), draft, nil
}
