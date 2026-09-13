package platform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOriginPolicyRequiresExactTrustedSchemeHostAndPort(t *testing.T) {
	policy, err := newOriginPolicy(Config{PublicOrigin: "https://academy.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		origin string
		allow  bool
	}{
		{"https://academy.example.com", true},
		{"https://academy.example.com:443", true},
		{"https://academy.example.com:8443", false},
		{"http://academy.example.com", false},
		{"https://academy.example.com.attacker.test", false},
		{"https://attacker.test/academy.example.com", false},
		{"null", false},
		{"", false},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		if scenario.origin != "" {
			r.Header.Set("Origin", scenario.origin)
		}
		r.Host = "attacker.test"
		r.Header.Set("X-Forwarded-Host", "academy.example.com")
		if got := policy.allowsRequest(r); got != scenario.allow {
			t.Fatalf("origin policy accepted=%t for %q", got, scenario.origin)
		}
	}
	for _, safe := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if !policy.allowsRequest(httptest.NewRequest(safe, "/api/auth/session", nil)) {
			t.Fatalf("safe method %s required Origin", safe)
		}
	}
	for _, unsafe := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if policy.allowsRequest(httptest.NewRequest(unsafe, "/api/example", nil)) {
			t.Fatalf("unsafe method %s skipped Origin", unsafe)
		}
	}
	duplicate := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	duplicate.Header.Add("Origin", "https://academy.example.com")
	duplicate.Header.Add("Origin", "https://academy.example.com")
	if policy.allowsRequest(duplicate) {
		t.Fatal("duplicate Origin headers were accepted")
	}
	if _, err := newOriginPolicy(Config{PublicOrigin: "http://academy.example.com"}); err == nil {
		t.Fatal("production accepted an HTTP public origin")
	}
	for _, malformed := range []string{"https://academy.example.com/path", "https://academy.example.com?x=1", "https://academy.example.com#fragment", "https://user@academy.example.com", "https://academy.example.com:99999", "https://academy.example.com/"} {
		if _, err := newOriginPolicy(Config{PublicOrigin: malformed}); err == nil {
			t.Fatal("malformed public origin was accepted")
		}
	}
}

func TestDevelopmentOriginPolicyAllowsOnlyConfiguredViteAndBackendLoopback(t *testing.T) {
	policy, err := newOriginPolicy(Config{PublicOrigin: "http://localhost:5173", HTTPAddr: "127.0.0.1:8080", DevelopmentHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, allowed := range []string{"http://localhost:5173", "http://127.0.0.1:8080"} {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		r.Header.Set("Origin", allowed)
		if !policy.allowsRequest(r) {
			t.Fatal("configured loopback origin was rejected")
		}
	}
	for _, rejected := range []string{"http://localhost:8080", "http://127.0.0.1:5173", "https://localhost:5173"} {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		r.Header.Set("Origin", rejected)
		if policy.allowsRequest(r) {
			t.Fatal("unconfigured origin was accepted")
		}
	}
	if _, err := newOriginPolicy(Config{PublicOrigin: "http://attacker.test", HTTPAddr: "127.0.0.1:8080", DevelopmentHTTP: true}); err == nil {
		t.Fatal("development trusted non-loopback public origin")
	}
}

func TestCrossOriginAndMissingOriginLoginNeverReachAuthentication(t *testing.T) {
	login := &authLoginFake{}
	router := authTestRouter(&authHTTP{login: login, now: func() time.Time { return time.Now() }})
	for _, origin := range []string{"", "https://attacker.test", "http://academy.example.com"} {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"private passphrase"}`))
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || login.loginCalls != 0 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "private passphrase") {
			t.Fatal("login CSRF origin rejection leaked or authenticated")
		}
	}
}
