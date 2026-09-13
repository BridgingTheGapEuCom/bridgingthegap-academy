//go:build integration

package platform

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testHTTPAuthorizationBoundary(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repository := identitypostgres.New(pool)
	user, err := repository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateUserEmail(ctx, user.ID, "policy-test@example.com", true); err != nil {
		t.Fatal(err)
	}
	hasher, err := identity.NewArgon2idHasher(identity.Argon2idParameters{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := hasher.HashPassword([]byte("correct horse battery staple"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateLocalPasswordCredential(ctx, user.ID, hash); err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(repository, nil, nil)
	auth := &authHTTP{
		login:      NewPostgresLoginOrchestrator(pool, nil, nil),
		sessions:   sessions,
		csrf:       identity.NewSessionCSRFService(repository, nil),
		authorizer: identity.NewAuthorizationService(repository),
		now:        time.Now,
	}
	router := authTestRouter(auth)
	login := authRequest(router, http.MethodPost, "/api/auth/login", `{"email":"policy-test@example.com","password":"correct horse battery staple"}`, nil)
	if login.Code != http.StatusOK || len(login.Result().Cookies()) != 1 {
		t.Fatal("non-administrator could not establish a valid session")
	}
	cookie := login.Result().Cookies()[0]
	adminStatus := func() int { return authRequest(router, http.MethodGet, "/api/admin/status", "", cookie).Code }
	if got := adminStatus(); got != http.StatusForbidden {
		t.Fatalf("authenticated non-administrator was not denied: %d", got)
	}
	if _, err := repository.AssignGlobalRole(ctx, user.ID, identity.RoleAdministrator, nil); err != nil {
		t.Fatal(err)
	}
	if got := adminStatus(); got != http.StatusOK {
		t.Fatalf("new role did not authorize same session on next request: %d", got)
	}
	if _, err := repository.RevokeGlobalRole(ctx, user.ID, identity.RoleAdministrator); err != nil {
		t.Fatal(err)
	}
	if got := adminStatus(); got != http.StatusForbidden {
		t.Fatalf("revoked role remained authorized on same session: %d", got)
	}
	// A different user's active Administrator assignment has no effect here.
	adminEmail, err := repository.GetUserEmailByNormalized(ctx, "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	adminSession, err := sessions.CreateSession(ctx, adminEmail.UserID)
	if err != nil {
		t.Fatal(err)
	}
	adminCookie := &http.Cookie{Name: sessionCookieName, Value: adminSession.Token.Value()}
	if got := authRequest(router, http.MethodGet, "/api/admin/status", "", adminCookie); got.Code != http.StatusOK || adminStatus() != http.StatusForbidden {
		t.Fatal("one user's administrator role leaked to another user")
	}
	if _, err := repository.UpdateUserStatus(ctx, adminEmail.UserID, identity.UserSuspended); err != nil {
		t.Fatal(err)
	}
	if got := authRequest(router, http.MethodGet, "/api/admin/status", "", adminCookie); got.Code != http.StatusUnauthorized {
		t.Fatal("suspended administrator reached authorization instead of failing authentication")
	}
	if _, err := repository.UpdateUserStatus(ctx, adminEmail.UserID, identity.UserActive); err != nil {
		t.Fatal(err)
	}
}
