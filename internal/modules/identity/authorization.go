package identity

import (
	"context"
	"errors"
)

var ErrAuthorizationDenied = errors.New("authorization denied")
var ErrAuthorizationUnavailable = errors.New("authorization unavailable")

// AuthenticatedActor carries identity only. The session resolver must run on
// each request; roles and capabilities are never copied into this value.
type AuthenticatedActor struct{ session ResolvedSession }

func ActorFromResolvedSession(session ResolvedSession) (AuthenticatedActor, error) {
	if session.userID == "" || session.sessionID == "" {
		return AuthenticatedActor{}, ErrAuthorizationDenied
	}
	return AuthenticatedActor{session: session}, nil
}

func (a AuthenticatedActor) UserID() UserID       { return a.session.userID }
func (a AuthenticatedActor) SessionID() SessionID { return a.session.sessionID }

type Capability string

const CapabilityInstanceManage Capability = "instance.manage"

// Resource has a narrow instance constructor now. Domain modules can later
// contribute resource-specific policies without making Administrator a bypass.
type Resource struct{ kind string }

func InstanceResource() Resource { return Resource{kind: "instance"} }

type Authorizer interface {
	Authorize(context.Context, AuthenticatedActor, Capability, Resource) error
}

// ActiveGlobalRoleReader is the focused Identity repository read needed by
// the first coarse global policy. The PostgreSQL adapter already implements it.
type ActiveGlobalRoleReader interface {
	HasActiveGlobalRole(context.Context, UserID, GlobalRole) (bool, error)
}

type AuthorizationService struct{ roles ActiveGlobalRoleReader }

func NewAuthorizationService(roles ActiveGlobalRoleReader) AuthorizationService {
	return AuthorizationService{roles: roles}
}

func (s AuthorizationService) Authorize(ctx context.Context, actor AuthenticatedActor, capability Capability, resource Resource) error {
	if actor.UserID() == "" || actor.SessionID() == "" {
		return ErrAuthorizationDenied
	}
	if capability != CapabilityInstanceManage || resource != InstanceResource() {
		return ErrAuthorizationDenied
	}
	if s.roles == nil {
		return ErrAuthorizationUnavailable
	}
	active, err := s.roles.HasActiveGlobalRole(ctx, actor.UserID(), RoleAdministrator)
	if err != nil {
		return ErrAuthorizationUnavailable
	}
	if !active {
		return ErrAuthorizationDenied
	}
	return nil
}
