//go:build integration

package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testHTTPLoginAdmission(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	var sessionsBefore, auditsBefore int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions").Scan(&sessionsBefore); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit.events WHERE action = 'LOCAL_PASSWORD_LOGIN_COMPLETED'").Scan(&auditsBefore); err != nil {
		t.Fatal(err)
	}
	service := NewPostgresLoginOrchestrator(pool, nil, nil)
	policy := testLoginRatePolicy()
	policy.burst = 1
	auth := &authHTTP{login: service, now: time.Now, loginSources: newLoginSourceLimiter(policy, nil), loginWork: newLoginWorkGuard(2)}
	router := authTestRouter(auth)
	first := loginFromPeerWithPassword(router, "192.0.2.60:1111", "admin@example.com", "correct horse battery staple", "https://academy.example.com")
	if first.Code != http.StatusOK || len(first.Result().Cookies()) != 1 {
		t.Fatalf("PostgreSQL-backed login failed below admission limit: status=%d", first.Code)
	}
	limited := loginFromPeerWithPassword(router, "192.0.2.60:2222", "admin@example.com", "correct horse battery staple", "https://academy.example.com")
	if limited.Code != http.StatusTooManyRequests || len(limited.Result().Cookies()) != 0 {
		t.Fatal("PostgreSQL-backed rate-limited attempt created a cookie")
	}
	var sessionsAfter, auditsAfter int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions").Scan(&sessionsAfter); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit.events WHERE action = 'LOCAL_PASSWORD_LOGIN_COMPLETED'").Scan(&auditsAfter); err != nil {
		t.Fatal(err)
	}
	if sessionsAfter != sessionsBefore+1 || auditsAfter != auditsBefore+1 {
		t.Fatal("rejected login created a session or completed-login audit fact")
	}

	// Two admitted requests can finish independently without sharing a session.
	concurrent := &authHTTP{login: service, now: time.Now, loginSources: newLoginSourceLimiter(defaultLoginRatePolicy, nil), loginWork: newLoginWorkGuard(2)}
	concurrentRouter := authTestRouter(concurrent)
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() {
			<-start
			responses <- loginFromPeerWithPassword(concurrentRouter, "192.0.2.61:1111", "admin@example.com", "correct horse battery staple", "https://academy.example.com")
		}()
	}
	close(start)
	second, third := <-responses, <-responses
	if second.Code != http.StatusOK || third.Code != http.StatusOK || len(second.Result().Cookies()) != 1 || len(third.Result().Cookies()) != 1 || second.Result().Cookies()[0].Value == third.Result().Cookies()[0].Value {
		t.Fatal("concurrent admitted logins failed to create independent sessions")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions").Scan(&sessionsAfter); err != nil || sessionsAfter != sessionsBefore+3 {
		t.Fatal("concurrent login session count does not match committed results")
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit.events WHERE action = 'LOCAL_PASSWORD_LOGIN_COMPLETED'").Scan(&auditsAfter); err != nil || auditsAfter != auditsBefore+3 {
		t.Fatal("concurrent login audit count does not match committed results")
	}
}
