//go:build integration

package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type passwordAuthenticationAttempts struct {
	events []identity.PasswordAuthenticationAttempt
}

func (s *passwordAuthenticationAttempts) ObservePasswordAuthentication(_ context.Context, attempt identity.PasswordAuthenticationAttempt) {
	s.events = append(s.events, attempt)
}

func testPasswordAuthentication(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repository := identitypostgres.New(pool)
	adminEmail, err := repository.GetUserEmailByNormalized(ctx, "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	beforeUser, err := repository.GetUser(ctx, adminEmail.UserID)
	if err != nil {
		t.Fatal(err)
	}
	beforeCredential, err := repository.GetLocalPasswordCredential(ctx, adminEmail.UserID)
	if err != nil {
		t.Fatal(err)
	}
	observer := &passwordAuthenticationAttempts{}
	service := identity.NewPasswordAuthenticator(repository, identity.DefaultPasswordHasher(), observer, nil)

	authenticated, err := service.AuthenticatePassword(ctx, " ADMIN@Example.COM ", []byte("correct horse battery staple"))
	if err != nil || authenticated.UserID != adminEmail.UserID || authenticated.Method != identity.AuthenticationMethodLocalPassword {
		t.Fatalf("bootstrapped ACTIVE account did not authenticate: %+v, %v", authenticated, err)
	}
	if len(observer.events) != 1 || observer.events[0].Outcome != identity.PasswordAuthenticationSucceeded {
		t.Fatalf("success security signal missing: %+v", observer.events)
	}

	for _, attempt := range []struct{ email, password string }{
		{"admin@example.com", "wrong horse battery staple"},
		{"nobody@example.com", "correct horse battery staple"},
	} {
		result, err := service.AuthenticatePassword(ctx, attempt.email, []byte(attempt.password))
		if err != identity.ErrInvalidCredentials || result != (identity.AuthenticatedIdentity{}) {
			t.Fatalf("ordinary failure was not generic: %+v, %v", result, err)
		}
	}
	if observer.events[2].Outcome != identity.PasswordAuthenticationUnknownAccount || observer.events[2].UserID != "" {
		t.Fatalf("unknown account security signal exposed a user ID: %+v", observer.events[2])
	}

	missingCredentialUser, err := repository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateUserEmail(ctx, missingCredentialUser.ID, "no-password@example.com", true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticatePassword(ctx, "no-password@example.com", []byte("correct horse battery staple")); err != identity.ErrInvalidCredentials {
		t.Fatalf("missing local credential was not generic: %v", err)
	}

	corruptUser, err := repository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateUserEmail(ctx, corruptUser.ID, "corrupt@example.com", true); err != nil {
		t.Fatal(err)
	}
	badHash, err := identity.NewPasswordHash("malformed-phc-value")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateLocalPasswordCredential(ctx, corruptUser.ID, badHash); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticatePassword(ctx, "corrupt@example.com", []byte("correct horse battery staple")); !errors.Is(err, identity.ErrCorruptPasswordCredential) {
		t.Fatalf("corrupt stored hash was not an operational failure: %v", err)
	}

	if _, err := repository.UpdateUserStatus(ctx, adminEmail.UserID, identity.UserSuspended); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticatePassword(ctx, "admin@example.com", []byte("correct horse battery staple")); err != identity.ErrInvalidCredentials {
		t.Fatalf("suspended user authenticated or received distinct error: %v", err)
	}
	if observer.events[len(observer.events)-1].Outcome != identity.PasswordAuthenticationSuspendedAccount {
		t.Fatal("suspended-account security signal missing")
	}
	if _, err := repository.UpdateUserStatus(ctx, adminEmail.UserID, identity.UserActive); err != nil {
		t.Fatal(err)
	}

	afterUser, err := repository.GetUser(ctx, adminEmail.UserID)
	if err != nil {
		t.Fatal(err)
	}
	afterCredential, err := repository.GetLocalPasswordCredential(ctx, adminEmail.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if afterUser.ID != beforeUser.ID || afterUser.Status != beforeUser.Status || afterUser.CreatedAt != beforeUser.CreatedAt || afterCredential.PasswordHash.Value() != beforeCredential.PasswordHash.Value() || afterCredential.UpdatedAt != beforeCredential.UpdatedAt {
		t.Fatal("authentication unexpectedly changed the user or credential")
	}
	var sessionCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", adminEmail.UserID).Scan(&sessionCount); err != nil || sessionCount != 0 {
		t.Fatalf("authentication created a session: count=%d err=%v", sessionCount, err)
	}
}
