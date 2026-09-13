//go:build integration

package platform

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit"
	auditpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testCompletedLoginLogout(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repository := identitypostgres.New(pool)
	email, err := repository.GetUserEmailByNormalized(ctx, "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	service := NewPostgresLoginOrchestrator(pool, nil, nil)
	sessions := identity.NewSessionService(repository, nil, nil)
	operationID := "00000000-0000-0000-0000-000000000101"
	password := "correct horse battery staple"
	login, err := service.LoginWithPassword(ctx, " ADMIN@Example.com ", []byte(password), operationID)
	if err != nil || login.UserID != email.UserID || login.Token.Value() == "" || login.Method != identity.AuthenticationMethodLocalPassword {
		t.Fatalf("bootstrapped account did not complete login: %v", err)
	}
	current, err := sessions.ResolveSession(ctx, login.Token.Value())
	if err != nil || current.SessionID() != login.SessionID {
		t.Fatalf("completed-login bearer token did not resolve: %v", err)
	}
	events, err := auditpostgres.New(pool).ListForResource(ctx, "IDENTITY_SESSION", string(login.SessionID))
	if err != nil || len(events) != 1 || events[0].Action != audit.LocalPasswordLoginCompleted || events[0].ActorUserID != string(email.UserID) || events[0].AuthenticationMethod != "LOCAL_PASSWORD" || events[0].OperationID != operationID || events[0].OccurredAt.IsZero() {
		t.Fatalf("completed-login audit fact missing: %+v err=%v", events, err)
	}

	var countBefore int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", email.UserID).Scan(&countBefore); err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []struct{ email, password string }{
		{"admin@example.com", "wrong horse battery staple"},
		{"unknown@example.com", password},
	} {
		result, err := service.LoginWithPassword(ctx, attempt.email, []byte(attempt.password), "")
		if err != identity.ErrInvalidCredentials || result != (LoginResult{}) {
			t.Fatalf("invalid credential became completed login: %v", err)
		}
	}
	if _, err := repository.UpdateUserStatus(ctx, email.UserID, identity.UserSuspended); err != nil {
		t.Fatal(err)
	}
	if _, err := service.LoginWithPassword(ctx, "admin@example.com", []byte(password), ""); err != identity.ErrInvalidCredentials {
		t.Fatalf("suspended account completed login: %v", err)
	}
	if _, err := repository.UpdateUserStatus(ctx, email.UserID, identity.UserActive); err != nil {
		t.Fatal(err)
	}
	var countAfterInvalid int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", email.UserID).Scan(&countAfterInvalid); err != nil || countAfterInvalid != countBefore {
		t.Fatalf("invalid credentials created session: count=%d err=%v", countAfterInvalid, err)
	}

	// The session insert happens first. Reusing a committed Audit operation ID
	// makes the append fail and proves the transaction rolls the insert back.
	failed, err := service.LoginWithPassword(ctx, "admin@example.com", []byte(password), operationID)
	if err != ErrLoginUnavailable || failed != (LoginResult{}) {
		t.Fatalf("late audit failure reported successful login: %v", err)
	}
	var countAfterRollback int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", email.UserID).Scan(&countAfterRollback); err != nil || countAfterRollback != countBefore {
		t.Fatalf("audit failure left an unaudited session: count=%d err=%v", countAfterRollback, err)
	}
	// Force the session step to fail after an otherwise successful application
	// authentication result, without changing production authentication code.
	missingUser := identity.UserID("00000000-0000-0000-0000-00000000ffff")
	missingActor := &loginAuthenticatorFake{actor: identity.AuthenticatedIdentity{UserID: missingUser, Method: identity.AuthenticationMethodLocalPassword, AuthenticatedAt: login.AuthenticatedAt}}
	missingService := NewLoginOrchestrator(missingActor, postgresCompletedSessionTransactionRunner{pool: pool})
	missingOperation := "00000000-0000-0000-0000-000000000106"
	if result, err := missingService.LoginWithPassword(ctx, "unused@example.com", []byte(password), missingOperation); err != ErrLoginUnavailable || result != (LoginResult{}) {
		t.Fatalf("session creation failure reported login success: %v", err)
	}
	var missingSessionCount, missingAuditCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", missingUser).Scan(&missingSessionCount); err != nil || missingSessionCount != 0 {
		t.Fatalf("failed session creation wrote a row: count=%d err=%v", missingSessionCount, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit.events WHERE operation_id = $1", missingOperation).Scan(&missingAuditCount); err != nil || missingAuditCount != 0 {
		t.Fatalf("failed session creation wrote a completed audit fact: count=%d err=%v", missingAuditCount, err)
	}

	concurrent, err := service.LoginWithPassword(ctx, "admin@example.com", []byte(password), "00000000-0000-0000-0000-000000000102")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(ctx, current, operationID); err != ErrLogoutUnavailable {
		t.Fatalf("late logout audit failure was not operational: %v", err)
	}
	if _, err := sessions.ResolveSession(ctx, login.Token.Value()); err != nil {
		t.Fatalf("audit failure revoked session despite rollback: %v", err)
	}
	if err := service.Logout(ctx, current, "00000000-0000-0000-0000-000000000103"); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.ResolveSession(ctx, login.Token.Value()); err != identity.ErrInvalidSession {
		t.Fatalf("logged-out session still resolves: %v", err)
	}
	if _, err := sessions.ResolveSession(ctx, concurrent.Token.Value()); err != nil {
		t.Fatalf("normal logout revoked another session: %v", err)
	}
	if err := service.Logout(ctx, current, "00000000-0000-0000-0000-000000000104"); err != nil {
		t.Fatalf("repeated logout failed: %v", err)
	}
	if err := service.Logout(ctx, current, "00000000-0000-0000-0000-000000000103"); err != nil {
		t.Fatalf("same-operation retry failed: %v", err)
	}
	events, err = auditpostgres.New(pool).ListForResource(ctx, "IDENTITY_SESSION", string(login.SessionID))
	if err != nil || len(events) != 2 || events[0].Action != audit.SessionLogoutCompleted || events[1].Action != audit.LocalPasswordLoginCompleted {
		t.Fatalf("logout audit facts missing: %+v err=%v", events, err)
	}
	for _, event := range events {
		if event.ActorUserID != string(email.UserID) || event.ResourceID != string(login.SessionID) || event.Outcome != "SUCCESS" {
			t.Fatalf("audit actor/session context incorrect: %+v", event)
		}
	}
	var auditJSON string
	if err := pool.QueryRow(ctx, "SELECT string_agg(row_to_json(e)::text, ' ') FROM audit.events e WHERE resource_type = 'IDENTITY_SESSION' AND resource_id = $1", login.SessionID).Scan(&auditJSON); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(login.Token.Value()))
	if strings.Contains(auditJSON, login.Token.Value()) || strings.Contains(auditJSON, password) || strings.Contains(auditJSON, fmt.Sprintf("%x", digest)) {
		t.Fatal("completed audit record contains bearer token, password, or token digest")
	}
	if _, err := sessions.ResolveSession(ctx, concurrent.Token.Value()); err != nil {
		t.Fatalf("other session stopped resolving after repeated logout: %v", err)
	}
	// Two completed-logout attempts may race on the same session row. PostgreSQL
	// serializes the conditional UPDATE; only the winner records completion.
	otherCurrent, err := sessions.ResolveSession(ctx, concurrent.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	outcomes := make(chan error, 2)
	for _, operation := range []string{"00000000-0000-0000-0000-000000000107", "00000000-0000-0000-0000-000000000108"} {
		go func(id string) { outcomes <- service.Logout(ctx, otherCurrent, id) }(operation)
	}
	for range 2 {
		if err := <-outcomes; err != nil {
			t.Fatalf("concurrent logout failed: %v", err)
		}
	}
	if _, err := sessions.ResolveSession(ctx, concurrent.Token.Value()); err != identity.ErrInvalidSession {
		t.Fatalf("concurrently logged-out session still resolves: %v", err)
	}
	otherEvents, err := auditpostgres.New(pool).ListForResource(ctx, "IDENTITY_SESSION", string(concurrent.SessionID))
	if err != nil || len(otherEvents) != 2 || otherEvents[0].Action != audit.SessionLogoutCompleted || otherEvents[1].Action != audit.LocalPasswordLoginCompleted {
		t.Fatalf("concurrent logout did not record exactly one completion: %+v err=%v", otherEvents, err)
	}
	if err := service.Logout(ctx, identity.ResolvedSession{}, ""); !errors.Is(err, identity.ErrInvalidSession) {
		t.Fatalf("empty trusted context accepted: %v", err)
	}
}
