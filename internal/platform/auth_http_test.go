package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

type authLoginFake struct {
	result      LoginResult
	err         error
	loginCalls  int
	logoutCalls int
	operationID string
	logoutID    identity.SessionID
}

func (f *authLoginFake) LoginWithPassword(_ context.Context, _ string, _ []byte, operationID string) (LoginResult, error) {
	f.loginCalls++
	f.operationID = operationID
	return f.result, f.err
}

func (f *authLoginFake) Logout(_ context.Context, current identity.ResolvedSession, operationID string) error {
	f.logoutCalls++
	f.logoutID = current.SessionID()
	f.operationID = operationID
	return f.err
}

type authResolverFake struct {
	current identity.ResolvedSession
	err     error
	calls   int
}

func (f *authResolverFake) ResolveSession(_ context.Context, _ string) (identity.ResolvedSession, error) {
	f.calls++
	return f.current, f.err
}

func authTestRouter(auth *authHTTP) http.Handler {
	return newRouter(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), prometheus.NewCounterVec(prometheus.CounterOpts{Name: "auth_test_requests_total"}, []string{"route", "method", "status_class"}), prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "auth_test_duration_seconds"}, []string{"route", "method"}), auth)
}

func authRequest(router http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func TestHTTPLoginCookiePolicyAndTrustedOperationID(t *testing.T) {
	at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	for _, secure := range []bool{true, false} {
		login := &authLoginFake{result: LoginResult{UserID: loginTestUser, Token: mustRawToken(), ExpiresAt: at.Add(time.Hour)}}
		router := authTestRouter(&authHTTP{login: login, cookieSecure: secure, now: func() time.Time { return at }})
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"private passphrase"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Request-ID", "00000000-0000-0000-0000-000000000999")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || login.loginCalls != 1 {
			t.Fatalf("login failed: status=%d", response.Code)
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != sessionCookieName || !cookies[0].HttpOnly || cookies[0].Secure != secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/" || cookies[0].MaxAge != 3600 || !cookies[0].Expires.Equal(at.Add(time.Hour)) {
			t.Fatal("session cookie attributes do not match server policy")
		}
		if cookies[0].Value != login.result.Token.Value() || strings.Contains(response.Body.String(), login.result.Token.Value()) || strings.Contains(response.Body.String(), "private passphrase") || strings.Contains(response.Header().Get("Set-Cookie"), string(loginTestUser)) {
			t.Fatal("login transport exposed a secret or account claim")
		}
		parsed, err := uuid.Parse(response.Header().Get("X-Request-ID"))
		if err != nil || parsed.String() != uuid.MustParse(login.operationID).String() || login.operationID == "00000000-0000-0000-0000-000000000999" {
			t.Fatal("client request ID overrode trusted operation ID")
		}
		var body authenticatedSessionResponse
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || !body.Authenticated || body.UserID != loginTestUser || !body.ExpiresAt.Equal(at.Add(time.Hour)) {
			t.Fatal("minimal login response is incorrect")
		}
	}
}

func TestHTTPLoginErrorsAreGenericAndBounded(t *testing.T) {
	for _, label := range []string{"unknown account", "wrong password", "suspended account"} {
		login := &authLoginFake{err: identity.ErrInvalidCredentials}
		response := authRequest(authTestRouter(&authHTTP{login: login, now: time.Now}), http.MethodPost, "/api/auth/login", `{"email":"admin@example.com","password":"private passphrase"}`, nil)
		if response.Code != http.StatusUnauthorized || !bytes.Contains(response.Body.Bytes(), []byte(`"title":"Invalid credentials"`)) || strings.Contains(response.Body.String(), "private passphrase") || len(response.Result().Cookies()) != 0 {
			t.Fatalf("%s leaked account state or secret: status=%d", label, response.Code)
		}
	}
	login := &authLoginFake{err: ErrLoginUnavailable}
	response := authRequest(authTestRouter(&authHTTP{login: login, now: time.Now}), http.MethodPost, "/api/auth/login", `{"email":"admin@example.com","password":"private passphrase"}`, nil)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private") {
		t.Fatal("operational login failure leaked details")
	}
	for _, body := range []string{`{`, `{"email":"admin@example.com"}`, `{"email":"a","password":"x","extra":true}`, `{"email":"a","password":"x"} {}`, strings.Repeat("a", maxLoginBodyBytes+1)} {
		login := &authLoginFake{}
		response := authRequest(authTestRouter(&authHTTP{login: login, now: time.Now}), http.MethodPost, "/api/auth/login", body, nil)
		if response.Code != http.StatusBadRequest || login.loginCalls != 0 || response.Header().Get("Content-Type") != "application/problem+json" || response.Header().Get("X-Request-ID") == "" {
			t.Fatal("malformed login body reached authentication or lacked Problem Details")
		}
	}
}

func TestHTTPCurrentSessionMiddlewareAndLogout(t *testing.T) {
	current := loginTestCurrent(t)
	valid := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	for _, scenario := range []struct {
		name   string
		cookie *http.Cookie
		err    error
		status int
		calls  int
	}{
		{"no cookie", nil, nil, 401, 0},
		{"valid", valid, nil, 200, 1},
		{"malformed", &http.Cookie{Name: sessionCookieName, Value: "malformed"}, identity.ErrInvalidSession, 401, 1},
		{"unknown", valid, identity.ErrInvalidSession, 401, 1},
		{"revoked", valid, identity.ErrInvalidSession, 401, 1},
		{"expired", valid, identity.ErrInvalidSession, 401, 1},
		{"suspended", valid, identity.ErrInvalidSession, 401, 1},
		{"storage failure", valid, identity.ErrSessionUnavailable, 500, 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			resolver := &authResolverFake{current: current, err: scenario.err}
			response := authRequest(authTestRouter(&authHTTP{sessions: resolver}), http.MethodGet, "/api/auth/session", "", scenario.cookie)
			if response.Code != scenario.status || resolver.calls != scenario.calls {
				t.Fatalf("session resolution status=%d calls=%d", response.Code, resolver.calls)
			}
			if scenario.status == 200 && (!strings.Contains(response.Body.String(), string(current.UserID())) || strings.Contains(response.Body.String(), valid.Value)) {
				t.Fatal("current-session response leaked token or omitted user")
			}
			if scenario.status != 200 && strings.Contains(response.Body.String(), string(current.UserID())) {
				t.Fatal("failed resolution populated trusted context")
			}
		})
	}
	empty := &authResolverFake{}
	if got := authRequest(authTestRouter(&authHTTP{sessions: empty}), http.MethodGet, "/api/auth/session", "", valid); got.Code != http.StatusInternalServerError {
		t.Fatal("empty resolver result was trusted")
	}
	duplicateRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	duplicateRequest.AddCookie(valid)
	duplicateRequest.AddCookie(valid)
	duplicateResponse := httptest.NewRecorder()
	authTestRouter(&authHTTP{sessions: empty}).ServeHTTP(duplicateResponse, duplicateRequest)
	if duplicateResponse.Code != http.StatusUnauthorized || empty.calls != 1 {
		t.Fatal("duplicate session cookies were accepted")
	}
	for _, cookie := range []*http.Cookie{nil, {Name: sessionCookieName, Value: "malformed"}} {
		resolver := &authResolverFake{err: identity.ErrInvalidSession}
		login := &authLoginFake{}
		response := authRequest(authTestRouter(&authHTTP{login: login, sessions: resolver, cookieSecure: true}), http.MethodPost, "/api/auth/logout", "", cookie)
		cleared := response.Result().Cookies()
		if response.Code != http.StatusNoContent || login.logoutCalls != 0 || len(cleared) != 1 || cleared[0].MaxAge != -1 || !cleared[0].Secure || !cleared[0].HttpOnly || cleared[0].Path != "/" || cleared[0].SameSite != http.SameSiteLaxMode || !cleared[0].Expires.Equal(time.Unix(0, 0).UTC()) {
			t.Fatal("absent or invalid session was not safely cleared")
		}
	}
	resolver := &authResolverFake{current: current}
	login := &authLoginFake{}
	router := authTestRouter(&authHTTP{login: login, sessions: resolver, cookieSecure: true})
	response := authRequest(router, http.MethodPost, "/api/auth/logout", `{"session_id":"00000000-0000-0000-0000-000000000999"}`, valid)
	if response.Code != http.StatusNoContent || login.logoutCalls != 1 || login.logoutID != current.SessionID() || len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not use only the resolved current session")
	}
	if _, err := uuid.Parse(login.operationID); err != nil {
		t.Fatal("logout operation ID was not server generated")
	}
	login.err = errors.New("private storage detail")
	response = authRequest(router, http.MethodPost, "/api/auth/logout", "", valid)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private") || len(response.Result().Cookies()) != 0 {
		t.Fatal("failed logout cleared cookie or leaked storage error")
	}
}
