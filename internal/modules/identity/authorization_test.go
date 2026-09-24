package identity

import (
	"context"
	"errors"
	"testing"
)

type authorizationRoleFake struct {
	active bool
	err    error
	calls  int
	userID UserID
	role   GlobalRole
}

func (f *authorizationRoleFake) HasActiveGlobalRole(_ context.Context, userID UserID, role GlobalRole) (bool, error) {
	f.calls++
	f.userID, f.role = userID, role
	return f.active, f.err
}

func TestAuthorizationUsesCurrentAdministratorAssignment(t *testing.T) {
	actor, err := ActorFromResolvedSession(ResolvedSession{userID: "user-1", sessionID: "session-1"})
	if err != nil || actor.UserID() != "user-1" || actor.SessionID() != "session-1" {
		t.Fatal("trusted actor did not retain minimal session identity")
	}
	roles := &authorizationRoleFake{active: true}
	service := NewAuthorizationService(roles)
	if err := service.Authorize(context.Background(), actor, CapabilityInstanceManage, InstanceResource()); err != nil || roles.userID != actor.UserID() || roles.role != RoleAdministrator {
		t.Fatal("active administrator was not granted instance management")
	}
	if err := service.Authorize(context.Background(), actor, CapabilityPluginsManage, InstanceResource()); err != nil {
		t.Fatal("active administrator was not granted plugin management")
	}
	roles.active = false // The repository now reports the assignment revoked.
	if err := service.Authorize(context.Background(), actor, CapabilityInstanceManage, InstanceResource()); !errors.Is(err, ErrAuthorizationDenied) || roles.calls != 3 {
		t.Fatal("revoked administrator permission remained cached in the actor")
	}
	if actor.UserID() != "user-1" || actor.SessionID() != "session-1" {
		t.Fatal("authorization mutated actor identity")
	}
}

func TestAuthorizationDefaultsToDenyAndFailsClosed(t *testing.T) {
	roles := &authorizationRoleFake{active: true}
	service := NewAuthorizationService(roles)
	actor, _ := ActorFromResolvedSession(ResolvedSession{userID: "user-1", sessionID: "session-1"})
	for _, request := range []struct {
		actor      AuthenticatedActor
		capability Capability
		resource   Resource
	}{
		{AuthenticatedActor{}, CapabilityInstanceManage, InstanceResource()},
		{actor, Capability("unknown.capability"), InstanceResource()},
		{actor, CapabilityInstanceManage, Resource{}},
	} {
		if err := service.Authorize(context.Background(), request.actor, request.capability, request.resource); !errors.Is(err, ErrAuthorizationDenied) {
			t.Fatal("anonymous, unknown capability, or unknown resource was allowed")
		}
	}
	if roles.calls != 0 {
		t.Fatal("default-denied policy queried role persistence")
	}
	roles.err = errors.New("private role database detail")
	if err := service.Authorize(context.Background(), actor, CapabilityInstanceManage, InstanceResource()); !errors.Is(err, ErrAuthorizationUnavailable) || err.Error() == roles.err.Error() {
		t.Fatal("role-loading failure was not a safe operational error")
	}
	if err := NewAuthorizationService(nil).Authorize(context.Background(), actor, CapabilityInstanceManage, InstanceResource()); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatal("missing role reader allowed privileged access")
	}
	if _, err := ActorFromResolvedSession(ResolvedSession{}); !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatal("empty session constructed a trusted actor")
	}
}
