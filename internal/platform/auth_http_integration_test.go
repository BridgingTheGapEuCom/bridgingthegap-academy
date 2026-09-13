//go:build integration

package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit"
	auditpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testHTTPAuthTransport(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	now := time.Now().UTC()
	repository := identitypostgres.New(pool)
	service := NewPostgresLoginOrchestrator(pool, nil, func() time.Time { return now })
	auth := &authHTTP{login: service, sessions: identity.NewSessionService(repository, nil, func() time.Time { return now }), cookieSecure: true, now: func() time.Time { return now }}
	router := authTestRouter(auth)
	password := "correct horse battery staple"
	loginBody := `{"email":" ADMIN@Example.com ","password":"` + password + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "00000000-0000-0000-0000-000000000999")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(response.Result().Cookies()) != 1 {
		t.Fatalf("HTTP login did not complete: status=%d", response.Code)
	}
	cookie := response.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || !cookie.Expires.After(now) || strings.Contains(response.Body.String(), cookie.Value) || strings.Contains(response.Body.String(), password) {
		t.Fatal("HTTP login exposed a secret or set an insecure cookie")
	}
	var body authenticatedSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || !body.Authenticated || body.UserID == "" || body.ExpiresAt.IsZero() {
		t.Fatal("HTTP login response is incomplete")
	}
	parsedRequestID, err := uuid.Parse(response.Header().Get("X-Request-ID"))
	if err != nil || parsedRequestID.String() == "00000000-0000-0000-0000-000000000999" {
		t.Fatal("client supplied trusted operation ID")
	}
	current := authRequest(router, http.MethodGet, "/api/auth/session", "", cookie)
	if current.Code != http.StatusOK || !bytes.Contains(current.Body.Bytes(), []byte(string(body.UserID))) || strings.Contains(current.Body.String(), cookie.Value) {
		t.Fatal("current-session endpoint did not resolve the cookie")
	}
	issuedNow := now
	now = body.ExpiresAt
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", cookie); got.Code != http.StatusUnauthorized {
		t.Fatal("session was valid at the expiry boundary")
	}
	now = issuedNow
	var digest []byte
	var sessionID string
	if err := pool.QueryRow(ctx, "SELECT id, token_digest FROM identity.sessions WHERE user_id = $1 AND token_digest = $2", body.UserID, sha256Digest(cookie.Value)).Scan(&sessionID, &digest); err != nil || len(digest) != sha256.Size || bytes.Equal(digest, []byte(cookie.Value)) {
		t.Fatalf("session digest was not persisted correctly: %v", err)
	}
	loginEvents, err := auditpostgres.New(pool).ListForResource(ctx, "IDENTITY_SESSION", sessionID)
	if err != nil || len(loginEvents) != 1 || loginEvents[0].Action != audit.LocalPasswordLoginCompleted || loginEvents[0].OperationID != parsedRequestID.String() {
		t.Fatalf("completed login audit does not use server ID: %v", err)
	}

	var before int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", body.UserID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	var failures []map[string]any
	for _, bad := range []string{`{"email":"unknown@example.com","password":"` + password + `"}`, `{"email":"admin@example.com","password":"wrong passphrase"}`} {
		failure := authRequest(router, http.MethodPost, "/api/auth/login", bad, nil)
		if failure.Code != http.StatusUnauthorized || len(failure.Result().Cookies()) != 0 {
			t.Fatal("invalid credentials created an HTTP login")
		}
		var problemBody map[string]any
		if err := json.Unmarshal(failure.Body.Bytes(), &problemBody); err != nil {
			t.Fatal(err)
		}
		delete(problemBody, "request_id")
		failures = append(failures, problemBody)
	}
	if fmt.Sprint(failures[0]) != fmt.Sprint(failures[1]) {
		t.Fatal("unknown account and wrong password had different outward failures")
	}
	var after int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", body.UserID).Scan(&after); err != nil || after != before {
		t.Fatal("failed HTTP login created a session")
	}
	unknown := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", unknown); got.Code != http.StatusUnauthorized {
		t.Fatal("unknown cookie was accepted")
	}
	second := authRequest(router, http.MethodPost, "/api/auth/login", loginBody, nil)
	if second.Code != http.StatusOK {
		t.Fatal("second concurrent session could not be created")
	}
	otherCookie := second.Result().Cookies()[0]
	var secondBody authenticatedSessionResponse
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatal(err)
	}
	logout := authRequest(router, http.MethodPost, "/api/auth/logout", `{"session_id":"00000000-0000-0000-0000-000000000999"}`, cookie)
	if logout.Code != http.StatusNoContent || len(logout.Result().Cookies()) != 1 || logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("HTTP logout did not clear the current cookie")
	}
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", cookie); got.Code != http.StatusUnauthorized {
		t.Fatal("logged-out cookie still resolves")
	}
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", otherCookie); got.Code != http.StatusOK {
		t.Fatal("logout revoked another session")
	}
	if got := authRequest(router, http.MethodPost, "/api/auth/logout", "", cookie); got.Code != http.StatusNoContent {
		t.Fatal("already-invalid logout was not idempotent")
	}
	logoutEvents, err := auditpostgres.New(pool).ListForResource(ctx, "IDENTITY_SESSION", sessionID)
	if err != nil || len(logoutEvents) != 2 || logoutEvents[0].Action != audit.SessionLogoutCompleted {
		t.Fatal("HTTP logout did not append one completed audit fact")
	}
	var auditJSON string
	if err := pool.QueryRow(ctx, "SELECT string_agg(row_to_json(e)::text, ' ') FROM audit.events e WHERE resource_type = 'IDENTITY_SESSION' AND resource_id = $1", sessionID).Scan(&auditJSON); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditJSON, cookie.Value) || strings.Contains(auditJSON, password) || strings.Contains(auditJSON, fmt.Sprintf("%x", digest)) {
		t.Fatal("HTTP audit record contains credential or bearer material")
	}
	if _, err := repository.UpdateUserStatus(ctx, body.UserID, identity.UserSuspended); err != nil {
		t.Fatal(err)
	}
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", otherCookie); got.Code != http.StatusUnauthorized {
		t.Fatal("suspended user's cookie remained valid")
	}
	if _, err := repository.UpdateUserStatus(ctx, body.UserID, identity.UserActive); err != nil {
		t.Fatal(err)
	}
	now = secondBody.ExpiresAt
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", otherCookie); got.Code != http.StatusUnauthorized {
		t.Fatal("session was valid at the expiry boundary")
	}
}

func sha256Digest(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
