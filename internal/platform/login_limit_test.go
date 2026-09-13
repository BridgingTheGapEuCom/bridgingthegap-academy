package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

func testLoginRatePolicy() loginRatePolicy {
	return loginRatePolicy{burst: 2, refillInterval: 10 * time.Second, maxSources: 2, idle: 30 * time.Second, cleanupInterval: 5 * time.Second}
}

func TestLoginSourceTokenBucketAndBoundedState(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	limiter := newLoginSourceLimiter(testLoginRatePolicy(), func() time.Time { return now })
	first := "192.0.2.1:1234"
	if ok, _ := limiter.Allow(first); !ok {
		t.Fatal("first attempt was rejected")
	}
	if ok, _ := limiter.Allow("[::ffff:192.0.2.1]:9876"); !ok {
		t.Fatal("IPv4-mapped form did not use the same bucket")
	}
	if ok, retry := limiter.Allow(first); ok || retry != 10*time.Second {
		t.Fatalf("burst exceeded or retry delay wrong: allowed=%v retry=%s", ok, retry)
	}
	if ok, _ := limiter.Allow("[2001:db8::1]:1234"); !ok {
		t.Fatal("separate source shared the first bucket")
	}
	if ok, retry := limiter.Allow("192.0.2.2:1234"); ok || retry != 0 || len(limiter.buckets) != 2 {
		t.Fatal("full source map evicted an active bucket or grew past its cap")
	}
	now = now.Add(5 * time.Second)
	if ok, retry := limiter.Allow(first); ok || retry != 5*time.Second {
		t.Fatal("bucket did not refill predictably")
	}
	now = now.Add(5 * time.Second)
	if ok, _ := limiter.Allow(first); !ok {
		t.Fatal("refilled token was not admitted")
	}
	now = now.Add(31 * time.Second)
	if ok, _ := limiter.Allow("192.0.2.2:1234"); !ok || len(limiter.buckets) != 1 {
		t.Fatal("idle source state was not removed before admitting a new source")
	}
	for _, remote := range []string{"", "not-an-address", "example.com:1234", "192.0.2.3", "192.0.2.999:80", "192.0.2.3:bad", "192.0.2.3:70000"} {
		if ok, _ := limiter.Allow(remote); ok || len(limiter.buckets) != 1 {
			t.Fatal("malformed or non-IP remote peer bypassed limiter")
		}
	}
}

func TestLoginSourceMapNeverExceedsCapUnderAddressChurn(t *testing.T) {
	limiter := newLoginSourceLimiter(testLoginRatePolicy(), nil)
	for i := range 100 {
		remote := fmt.Sprintf("[2001:db8::%x]:1234", i+1)
		_, _ = limiter.Allow(remote)
		if len(limiter.buckets) > limiter.policy.maxSources {
			t.Fatal("source churn grew limiter state past hard cap")
		}
	}
}

func TestHTTPLoginWorkPermitReleasedForEveryOutcome(t *testing.T) {
	service := &authLoginFake{result: LoginResult{UserID: loginTestUser, Token: mustRawToken(), CSRFToken: authTestCSRFToken(), ExpiresAt: time.Now().Add(time.Hour)}}
	auth := &authHTTP{login: service, now: time.Now, loginSources: newLoginSourceLimiter(defaultLoginRatePolicy, nil), loginWork: newLoginWorkGuard(1)}
	router := authTestRouter(auth)
	for _, scenario := range []struct {
		err    error
		status int
	}{
		{nil, http.StatusOK},
		{identity.ErrInvalidCredentials, http.StatusUnauthorized},
		{ErrLoginUnavailable, http.StatusInternalServerError},
		{nil, http.StatusOK},
	} {
		service.err = scenario.err
		if got := loginFromPeer(router, "192.0.2.30:1234", "admin@example.com", "https://academy.example.com"); got.Code != scenario.status {
			t.Fatalf("login permit leaked after prior outcome: status=%d", got.Code)
		}
	}
}

func TestSuccessfulLoginDoesNotResetSourceAllowance(t *testing.T) {
	now := time.Now()
	service := &authLoginFake{result: LoginResult{UserID: loginTestUser, Token: mustRawToken(), CSRFToken: authTestCSRFToken(), ExpiresAt: now.Add(time.Hour)}}
	auth := &authHTTP{login: service, now: func() time.Time { return now }, loginSources: newLoginSourceLimiter(testLoginRatePolicy(), func() time.Time { return now }), loginWork: newLoginWorkGuard(1)}
	router := authTestRouter(auth)
	for range 2 {
		if got := loginFromPeer(router, "192.0.2.31:1234", "admin@example.com", "https://academy.example.com"); got.Code != http.StatusOK {
			t.Fatal("successful login was rejected within source burst")
		}
	}
	if got := loginFromPeer(router, "192.0.2.31:4321", "admin@example.com", "https://academy.example.com"); got.Code != http.StatusTooManyRequests || service.loginCalls != 2 {
		t.Fatal("successful login reset or bypassed the source bucket")
	}
}

type panicLoginService struct{}

func (panicLoginService) LoginWithPassword(context.Context, string, []byte, string) (LoginResult, error) {
	panic("test authentication failure")
}

func (panicLoginService) Logout(context.Context, identity.ResolvedSession, string) error { return nil }

func TestHTTPLoginPanicReleasesWorkPermit(t *testing.T) {
	guard := newLoginWorkGuard(1)
	auth := &authHTTP{login: panicLoginService{}, now: time.Now, loginSources: newLoginSourceLimiter(defaultLoginRatePolicy, nil), loginWork: guard}
	router := authTestRouter(auth)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("test login did not panic")
			}
		}()
		_ = loginFromPeer(router, "192.0.2.32:1234", "admin@example.com", "https://academy.example.com")
	}()
	if !guard.TryAcquire() {
		t.Fatal("panic permanently occupied authentication capacity")
	}
	guard.Release()
}

func TestLoginWorkGuardFailsFastAndReleasesCapacity(t *testing.T) {
	guard := newLoginWorkGuard(2)
	first, second, excess := guard.TryAcquire(), guard.TryAcquire(), guard.TryAcquire()
	if !first || !second || excess {
		t.Fatal("guard did not enforce its concurrent work cap")
	}
	guard.Release()
	if !guard.TryAcquire() {
		t.Fatal("released permit was not reusable")
	}
	guard.Release()
	guard.Release()
	first, second = guard.TryAcquire(), guard.TryAcquire()
	if !first || !second {
		t.Fatal("permits leaked after work completed")
	}
}

func loginFromPeer(router http.Handler, remote, email, origin string) *httptest.ResponseRecorder {
	return loginFromPeerWithPassword(router, remote, email, "private passphrase", origin)
}

func loginFromPeerWithPassword(router http.Handler, remote, email, password, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	request.RemoteAddr = remote
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", origin)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestHTTPLoginRateLimitIsSourceBasedAndPrecedesAuthentication(t *testing.T) {
	login := &authLoginFake{err: identity.ErrInvalidCredentials}
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	limiter := newLoginSourceLimiter(testLoginRatePolicy(), func() time.Time { return now })
	auth := &authHTTP{login: login, now: time.Now, loginSources: limiter, loginWork: newLoginWorkGuard(1)}
	router := authTestRouter(auth)
	peer := "192.0.2.10:1111"
	for _, email := range []string{"unknown@example.com", "admin@example.com"} {
		if got := loginFromPeer(router, peer, email, "https://academy.example.com"); got.Code != http.StatusUnauthorized {
			t.Fatal("request below source limit did not reach authentication")
		}
	}
	blocked := loginFromPeer(router, "192.0.2.10:9999", "admin@example.com", "https://academy.example.com")
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") != "10" || login.loginCalls != 2 || len(blocked.Result().Cookies()) != 0 {
		t.Fatal("source throttling did not stop authentication before session creation")
	}
	var body map[string]any
	if json.Unmarshal(blocked.Body.Bytes(), &body) != nil || body["title"] != "Too many requests" || strings.Contains(blocked.Body.String(), "admin@example.com") || strings.Contains(blocked.Body.String(), "private passphrase") {
		t.Fatal("rate-limit Problem Details exposed account or password data")
	}
	spoofed := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"private passphrase"}`))
	spoofed.RemoteAddr = peer
	spoofed.Header.Set("Content-Type", "application/json")
	spoofed.Header.Set("Origin", "https://academy.example.com")
	spoofed.Header.Set("X-Forwarded-For", "198.51.100.9")
	spoofed.Header.Set("X-Real-IP", "198.51.100.9")
	spoofedResult := httptest.NewRecorder()
	router.ServeHTTP(spoofedResult, spoofed)
	if spoofedResult.Code != http.StatusTooManyRequests || login.loginCalls != 2 {
		t.Fatal("client-controlled IP headers bypassed source limit")
	}
	if got := loginFromPeer(router, "198.51.100.9:1111", "admin@example.com", "https://evil.example.com"); got.Code != http.StatusForbidden || login.loginCalls != 2 {
		t.Fatal("untrusted Origin reached the limiter or authenticator")
	}
	if got := loginFromPeer(router, "198.51.100.9:1111", "admin@example.com", "https://academy.example.com"); got.Code != http.StatusUnauthorized || login.loginCalls != 3 {
		t.Fatal("independent source did not retain its allowance")
	}
}

type blockedLoginService struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (s *blockedLoginService) LoginWithPassword(_ context.Context, _ string, _ []byte, _ string) (LoginResult, error) {
	s.calls.Add(1)
	s.started <- struct{}{}
	<-s.release
	return LoginResult{}, identity.ErrInvalidCredentials
}

func (s *blockedLoginService) Logout(context.Context, identity.ResolvedSession, string) error {
	return nil
}

func TestHTTPLoginConcurrentWorkHasNoWaitingQueue(t *testing.T) {
	service := &blockedLoginService{started: make(chan struct{}, 2), release: make(chan struct{})}
	auth := &authHTTP{login: service, now: time.Now, loginSources: newLoginSourceLimiter(defaultLoginRatePolicy, nil), loginWork: newLoginWorkGuard(2)}
	router := authTestRouter(auth)
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() {
			results <- loginFromPeer(router, "192.0.2.20:1111", "admin@example.com", "https://academy.example.com")
		}()
	}
	<-service.started
	<-service.started
	if got := loginFromPeer(router, "192.0.2.20:1111", "unknown@example.com", "https://academy.example.com"); got.Code != http.StatusTooManyRequests || got.Header().Get("Retry-After") != "" || service.calls.Load() != 2 {
		t.Fatal("saturated login work queued or reached password authentication")
	}
	close(service.release)
	for range 2 {
		if got := <-results; got.Code != http.StatusUnauthorized {
			t.Fatal("admitted login did not finish after work release")
		}
	}
	if got := loginFromPeer(router, "192.0.2.20:1111", "admin@example.com", "https://academy.example.com"); got.Code != http.StatusUnauthorized || service.calls.Load() != 3 {
		t.Fatal("authentication work permit leaked after invalid credentials")
	}
}
