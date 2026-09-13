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
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testHTTPAuthTransport(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	now := time.Now().UTC()
	repository := identitypostgres.New(pool)
	service := NewPostgresLoginOrchestrator(pool, nil, func() time.Time { return now })
	auth := &authHTTP{login: service, sessions: identity.NewSessionService(repository, nil, func() time.Time { return now }), csrf: identity.NewSessionCSRFService(repository, func() time.Time { return now }), cookieSecure: true, now: func() time.Time { return now }}
	router := authTestRouter(auth)
	password := "correct horse battery staple"
	loginBody := `{"email":" ADMIN@Example.com ","password":"` + password + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://academy.example.com")
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
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || !body.Authenticated || body.UserID == "" || body.ExpiresAt.IsZero() || body.CSRFToken == "" || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Header().Get("Set-Cookie"), body.CSRFToken) {
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
	var currentBody authenticatedSessionResponse
	if err := json.Unmarshal(current.Body.Bytes(), &currentBody); err != nil || currentBody.CSRFToken != body.CSRFToken || current.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("current-session response did not deliver stable no-store CSRF token")
	}
	resolvedFirst, err := auth.sessions.ResolveSession(ctx, cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	issuedNow := now
	now = body.ExpiresAt
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", cookie); got.Code != http.StatusUnauthorized {
		t.Fatal("session was valid at the expiry boundary")
	}
	now = issuedNow
	var digest []byte
	var storedCSRF string
	var sessionID string
	if err := pool.QueryRow(ctx, "SELECT id, token_digest, csrf_token FROM identity.sessions WHERE user_id = $1 AND token_digest = $2", body.UserID, sha256Digest(cookie.Value)).Scan(&sessionID, &digest, &storedCSRF); err != nil || len(digest) != sha256.Size || bytes.Equal(digest, []byte(cookie.Value)) || storedCSRF != body.CSRFToken {
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
	// A browser that already has an authenticated cookie gets a fresh session
	// and synchronizer token; the supplied cookie cannot fix the new identity.
	second := authRequest(router, http.MethodPost, "/api/auth/login", loginBody, cookie)
	if second.Code != http.StatusOK {
		t.Fatal("second concurrent session could not be created")
	}
	otherCookie := second.Result().Cookies()[0]
	if otherCookie.Value == cookie.Value {
		t.Fatal("login reused the pre-existing browser session token")
	}
	var secondBody authenticatedSessionResponse
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil || secondBody.CSRFToken == body.CSRFToken {
		t.Fatal(err)
	}
	for _, rejected := range []string{"", "malformed", secondBody.CSRFToken} {
		attempt := authRequest(router, http.MethodPost, "/api/auth/logout", "", cookie, rejected)
		if attempt.Code != http.StatusForbidden || len(attempt.Result().Cookies()) != 0 || strings.Contains(attempt.Body.String(), body.CSRFToken) {
			t.Fatal("valid session logout accepted missing, malformed, or cross-session CSRF")
		}
		if got := authRequest(router, http.MethodGet, "/api/auth/session", "", cookie); got.Code != http.StatusOK {
			t.Fatal("CSRF rejection revoked session")
		}
	}
	logout := authRequest(router, http.MethodPost, "/api/auth/logout", `{"session_id":"00000000-0000-0000-0000-000000000999"}`, cookie, body.CSRFToken)
	if logout.Code != http.StatusNoContent || len(logout.Result().Cookies()) != 1 || logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("HTTP logout did not clear the current cookie")
	}
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", cookie); got.Code != http.StatusUnauthorized {
		t.Fatal("logged-out cookie still resolves")
	}
	if err := auth.csrf.Validate(ctx, resolvedFirst, body.CSRFToken); err != identity.ErrInvalidSession {
		t.Fatal("revoked session retained CSRF validation")
	}
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", otherCookie); got.Code != http.StatusOK {
		t.Fatal("logout revoked another session")
	}
	if got := authRequest(router, http.MethodPost, "/api/auth/logout", "", otherCookie, body.CSRFToken); got.Code != http.StatusForbidden {
		t.Fatal("old session CSRF token validated for another session")
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
	if strings.Contains(auditJSON, cookie.Value) || strings.Contains(auditJSON, password) || strings.Contains(auditJSON, fmt.Sprintf("%x", digest)) || strings.Contains(auditJSON, body.CSRFToken) || strings.Contains(auditJSON, secondBody.CSRFToken) {
		t.Fatal("HTTP audit record contains credential or bearer material")
	}
	protected := chi.NewRouter()
	protected.Use(correlationID)
	protected.Use(auth.enforceOrigin)
	reached := 0
	protected.With(auth.authenticated).Post("/protected", func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusNoContent)
	})
	if got := authRequest(protected, http.MethodPost, "/protected", "", otherCookie, secondBody.CSRFToken); got.Code != http.StatusNoContent || reached != 1 {
		t.Fatal("valid PostgreSQL-backed CSRF request did not reach protected handler")
	}
	if _, err := repository.UpdateUserStatus(ctx, body.UserID, identity.UserSuspended); err != nil {
		t.Fatal(err)
	}
	if got := authRequest(protected, http.MethodPost, "/protected", "", otherCookie, secondBody.CSRFToken); got.Code != http.StatusUnauthorized || reached != 1 {
		t.Fatal("suspended user reached protected mutation with old CSRF token")
	}
	if got := authRequest(router, http.MethodGet, "/api/auth/session", "", otherCookie); got.Code != http.StatusUnauthorized {
		t.Fatal("suspended user's cookie remained valid")
	}
	if _, err := repository.UpdateUserStatus(ctx, body.UserID, identity.UserActive); err != nil {
		t.Fatal(err)
	}
	// A session created before the CSRF migration has a NULL token. A safe
	// current-session GET provisions it once, without rotating on later GETs.
	legacyRaw := mustRawToken().Value()
	var legacySessionID string
	if err := pool.QueryRow(ctx, "INSERT INTO identity.sessions (user_id, token_digest, expires_at) VALUES ($1, $2, $3) RETURNING id", body.UserID, sha256Digest(legacyRaw), time.Now().Add(24*time.Hour)).Scan(&legacySessionID); err != nil {
		t.Fatal(err)
	}
	legacyCookie := &http.Cookie{Name: sessionCookieName, Value: legacyRaw}
	var legacyToken string
	for range 2 {
		got := authRequest(router, http.MethodGet, "/api/auth/session", "", legacyCookie)
		var legacyBody authenticatedSessionResponse
		if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &legacyBody) != nil || legacyBody.CSRFToken == "" {
			t.Fatal("legacy session could not provision CSRF token")
		}
		if legacyToken != "" && legacyBody.CSRFToken != legacyToken {
			t.Fatal("legacy CSRF token rotated on repeated GET")
		}
		legacyToken = legacyBody.CSRFToken
	}
	var persistedLegacy string
	if err := pool.QueryRow(ctx, "SELECT csrf_token FROM identity.sessions WHERE id = $1", legacySessionID).Scan(&persistedLegacy); err != nil || persistedLegacy != legacyToken {
		t.Fatal("legacy session CSRF token was not persisted")
	}
	// A context resolved before a status/expiry change must not be able to
	// provision or read CSRF state after that change.
	staleRaw := mustRawToken().Value()
	var staleID string
	if err := pool.QueryRow(ctx, "INSERT INTO identity.sessions (user_id, token_digest, expires_at) VALUES ($1, $2, $3) RETURNING id", body.UserID, sha256Digest(staleRaw), time.Now().Add(time.Hour)).Scan(&staleID); err != nil {
		t.Fatal(err)
	}
	stale, err := auth.sessions.ResolveSession(ctx, staleRaw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpdateUserStatus(ctx, body.UserID, identity.UserSuspended); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.csrf.TokenForSession(ctx, stale); err != identity.ErrInvalidSession {
		t.Fatal("suspended user's stale context provisioned CSRF state")
	}
	var unprovisioned *string
	if err := pool.QueryRow(ctx, "SELECT csrf_token FROM identity.sessions WHERE id = $1", staleID).Scan(&unprovisioned); err != nil || unprovisioned != nil {
		t.Fatal("suspended session received a CSRF token")
	}
	if _, err := repository.UpdateUserStatus(ctx, body.UserID, identity.UserActive); err != nil {
		t.Fatal(err)
	}
	startProvision := make(chan struct{})
	provisioned := make(chan identity.CSRFToken, 4)
	provisionErrors := make(chan error, 4)
	for range 4 {
		go func() {
			<-startProvision
			token, err := auth.csrf.TokenForSession(ctx, stale)
			provisioned <- token
			provisionErrors <- err
		}()
	}
	close(startProvision)
	var oneToken string
	for range 4 {
		token, err := <-provisioned, <-provisionErrors
		if err != nil || token.Value() == "" || oneToken != "" && token.Value() != oneToken {
			t.Fatal("concurrent legacy CSRF provisioning returned inconsistent tokens")
		}
		oneToken = token.Value()
	}
	if _, err := pool.Exec(ctx, "UPDATE identity.sessions SET created_at = now() - interval '2 seconds', expires_at = now() - interval '1 second' WHERE id = $1", staleID); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.csrf.TokenForSession(ctx, stale); err != identity.ErrInvalidSession {
		t.Fatal("expired session's stale context provisioned CSRF state")
	}
	if err := auth.csrf.Validate(ctx, stale, oneToken); err != identity.ErrInvalidSession {
		t.Fatal("expired session's stale context validated its old CSRF token")
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
