package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/go-chi/chi/v5"
)

type authorizationFake struct {
	err        error
	calls      int
	actor      identity.AuthenticatedActor
	capability identity.Capability
	resource   identity.Resource
}

func (f *authorizationFake) Authorize(_ context.Context, actor identity.AuthenticatedActor, capability identity.Capability, resource identity.Resource) error {
	f.calls++
	f.actor, f.capability, f.resource = actor, capability, resource
	return f.err
}

func TestAdminStatusAuthenticationAndAuthorizationBoundary(t *testing.T) {
	current := loginTestCurrent(t)
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	for _, scenario := range []struct {
		name       string
		cookie     *http.Cookie
		resolveErr error
		decideErr  error
		status     int
		calls      int
	}{
		{"anonymous", nil, nil, nil, 401, 0},
		{"invalid session", cookie, identity.ErrInvalidSession, nil, 401, 0},
		{"expired session", cookie, identity.ErrInvalidSession, nil, 401, 0},
		{"suspended account", cookie, identity.ErrInvalidSession, nil, 401, 0},
		{"no administrator role", cookie, nil, identity.ErrAuthorizationDenied, 403, 1},
		{"role load failure", cookie, nil, errors.New("private role persistence detail"), 500, 1},
		{"administrator", cookie, nil, nil, 200, 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			decision := &authorizationFake{err: scenario.decideErr}
			router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: current, err: scenario.resolveErr}, authorizer: decision})
			response := authRequest(router, http.MethodGet, "/api/admin/status", "", scenario.cookie)
			if response.Code != scenario.status || decision.calls != scenario.calls {
				t.Fatalf("unexpected admin boundary result: status=%d decisions=%d", response.Code, decision.calls)
			}
			if scenario.status == http.StatusOK {
				if !strings.Contains(response.Body.String(), `"status":"ok"`) || decision.actor.UserID() != current.UserID() || decision.actor.SessionID() != current.SessionID() || decision.capability != identity.CapabilityInstanceManage || decision.resource != identity.InstanceResource() {
					t.Fatal("administrator handler did not receive a trusted authorized actor")
				}
			} else if strings.Contains(response.Body.String(), "ADMINISTRATOR") || strings.Contains(response.Body.String(), "private role") || strings.Contains(response.Body.String(), string(current.UserID())) || response.Header().Get("Content-Type") != "application/problem+json" || response.Header().Get("X-Request-ID") == "" {
				t.Fatal("authorization failure exposed role details or lacked Problem Details")
			}
		})
	}
}

func TestAdminStatusCannotTrustClientIdentityHeaders(t *testing.T) {
	current := loginTestCurrent(t)
	decision := &authorizationFake{err: identity.ErrAuthorizationDenied}
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: current}, authorizer: decision})
	request := httptest.NewRequest(http.MethodGet, "/api/admin/status", nil)
	request.Header.Set("X-User-ID", "administrator")
	request.Header.Set("X-Role", "ADMINISTRATOR")
	request.Header.Set("X-Authenticated-User", "administrator")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || decision.calls != 1 || decision.actor.UserID() != current.UserID() || strings.Contains(response.Body.String(), "ADMINISTRATOR") {
		t.Fatal("client-supplied identity headers bypassed capability policy")
	}
}

func TestProtectedMutationChecksCSRFBeforeAuthorization(t *testing.T) {
	current := loginTestCurrent(t)
	decision := &authorizationFake{}
	auth := &authHTTP{sessions: &authResolverFake{current: current}, csrf: authCSRFFake{token: authTestCSRFToken()}, authorizer: decision}
	router := chi.NewRouter()
	router.Use(correlationID)
	router.With(auth.authenticated, auth.requireCapability(identity.CapabilityInstanceManage, identity.InstanceResource())).Post("/protected", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	if got := authRequest(router, http.MethodPost, "/protected", "", cookie); got.Code != http.StatusForbidden || decision.calls != 0 {
		t.Fatal("authorization ran before CSRF on a protected mutation")
	}
	if got := authRequest(router, http.MethodPost, "/protected", "", cookie, authTestCSRFToken().Value()); got.Code != http.StatusNoContent || decision.calls != 1 {
		t.Fatal("valid CSRF did not proceed to capability authorization")
	}
}
