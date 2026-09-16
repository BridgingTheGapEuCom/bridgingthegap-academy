package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/google/uuid"
)

type Capability string

const (
	CapabilityRead          Capability = "authoring.read"
	CapabilityDraftEdit     Capability = "authoring.draft.edit"
	CapabilityStructureEdit Capability = "authoring.structure.edit"
	CapabilityContentEdit   Capability = "authoring.content.edit"
	CapabilityMembersManage Capability = "authoring.members.manage"
	CapabilityDraftAbandon  Capability = "authoring.draft.abandon"
	CapabilityReviewRead    Capability = "authoring.review.read"
	CapabilityReviewSubmit  Capability = "authoring.review.submit"
	CapabilityReviewDecide  Capability = "authoring.review.decide"
	CapabilityPublish       Capability = "authoring.publish"
)

var (
	ErrAuthorizationDenied      = errors.New("authoring authorization denied")
	ErrAuthorizationUnavailable = errors.New("authoring authorization unavailable")
)

// Resource identifies the exact draft and its one-to-one workspace. A zero or
// malformed resource is never authorized. Future handlers may use a draft ID
// from the route, but cannot supply actor identity or membership claims here.
type Resource struct{ draftID DraftID }

func DraftResource(id DraftID) Resource { return Resource{draftID: id} }

type Authorizer interface {
	Authorize(context.Context, identity.AuthenticatedActor, Capability, Resource) error
}

// ActiveMembershipReader returns only the current role for this draft and user.
// Missing draft and missing membership both return found=false, without exposing
// historical membership rows or draft existence to the authorization caller.
type ActiveMembershipReader interface {
	ActiveMembershipForDraft(context.Context, DraftID, string) (MemberRole, bool, error)
}

type AuthorizationService struct{ memberships ActiveMembershipReader }

func NewAuthorizationService(memberships ActiveMembershipReader) AuthorizationService {
	return AuthorizationService{memberships: memberships}
}

func (s AuthorizationService) Authorize(ctx context.Context, actor identity.AuthenticatedActor, capability Capability, resource Resource) error {
	if actor.UserID() == "" || actor.SessionID() == "" {
		return ErrAuthorizationDenied
	}
	if _, err := uuid.Parse(string(resource.draftID)); err != nil || !knownCapability(capability) {
		return ErrAuthorizationDenied
	}
	if s.memberships == nil {
		return ErrAuthorizationUnavailable
	}
	role, found, err := s.memberships.ActiveMembershipForDraft(ctx, resource.draftID, string(actor.UserID()))
	if err != nil || (found && !role.Valid()) {
		return ErrAuthorizationUnavailable
	}
	if !found || !roleAllows(role, capability) {
		return ErrAuthorizationDenied
	}
	return nil
}

func knownCapability(capability Capability) bool {
	switch capability {
	case CapabilityRead, CapabilityDraftEdit, CapabilityStructureEdit,
		CapabilityContentEdit, CapabilityMembersManage, CapabilityDraftAbandon,
		CapabilityReviewRead, CapabilityReviewSubmit, CapabilityReviewDecide,
		CapabilityPublish:
		return true
	default:
		return false
	}
}

func roleAllows(role MemberRole, capability Capability) bool {
	switch role {
	case MemberAuthor:
		return capability == CapabilityRead || capability == CapabilityDraftEdit ||
			capability == CapabilityStructureEdit || capability == CapabilityContentEdit ||
			capability == CapabilityReviewRead || capability == CapabilityReviewSubmit
	case MemberMaintainer:
		return knownCapability(capability)
	default:
		return false
	}
}
