//go:build integration

package platform

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit"
	auditpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAdministratorBootstrap(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	hasher, err := identity.NewArgon2idHasher(identity.Argon2idParameters{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.users").Scan(&before); err != nil {
		t.Fatal(err)
	}
	options := AdministratorBootstrapOptions{Hasher: hasher, OperationID: "00000000-0000-0000-0000-000000000010"}
	password := []byte("correct horse battery staple")
	user, err := BootstrapAdministrator(ctx, pool, " Admin@Example.com ", password, options)
	if err != nil {
		t.Fatal(err)
	}
	var after int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.users").Scan(&after); err != nil || after != before+1 {
		t.Fatalf("bootstrap did not create exactly one user: before=%d after=%d err=%v", before, after, err)
	}
	if user.Status != identity.UserActive {
		t.Fatalf("administrator user is not active: %s", user.Status)
	}
	repository := identitypostgres.New(pool)
	email, err := repository.GetUserEmailByNormalized(ctx, "admin@example.COM")
	if err != nil || email.UserID != user.ID || email.NormalizedEmail != "admin@example.com" || !email.IsPrimary || email.VerifiedAt == nil {
		t.Fatalf("bootstrap email is incomplete: %+v err=%v", email, err)
	}
	credential, err := repository.GetLocalPasswordCredential(ctx, user.ID)
	if err != nil || !strings.HasPrefix(credential.PasswordHash.Value(), "$argon2id$") || strings.Contains(credential.PasswordHash.Value(), "correct horse battery staple") {
		t.Fatalf("bootstrap credential is invalid: %v", err)
	}
	role, err := repository.HasActiveGlobalRole(ctx, user.ID, identity.RoleAdministrator)
	if err != nil || !role {
		t.Fatalf("bootstrap administrator role missing: %v", err)
	}
	events, err := auditpostgres.New(pool).ListForResource(ctx, "IDENTITY_USER", string(user.ID))
	if err != nil || len(events) != 1 || events[0].Action != audit.AdministratorBootstrapped || events[0].ActorKind != "SYSTEM" || events[0].Outcome != "SUCCESS" || events[0].OperationID != options.OperationID {
		t.Fatalf("bootstrap audit event missing or invalid: %+v err=%v", events, err)
	}

	if _, err := BootstrapAdministrator(ctx, pool, "ADMIN@example.com", []byte("correct horse battery staple"), AdministratorBootstrapOptions{Hasher: hasher}); !errors.Is(err, identity.ErrAdministratorAlreadyExists) {
		t.Fatalf("duplicate bootstrap was not rejected safely: %v", err)
	}
	var afterDuplicate int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.users").Scan(&afterDuplicate); err != nil || afterDuplicate != after {
		t.Fatalf("duplicate bootstrap created a user: before=%d after=%d err=%v", after, afterDuplicate, err)
	}

	_, err = BootstrapAdministrator(ctx, pool, "rollback@example.com", []byte("correct horse battery staple"), options)
	if !errors.Is(err, identity.ErrAdministratorBootstrapFailed) {
		t.Fatalf("late audit failure did not fail bootstrap: %v", err)
	}
	if _, err := repository.GetUserEmailByNormalized(ctx, "rollback@example.com"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("late audit failure left an email behind: %v", err)
	}
	var afterRollback int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.users").Scan(&afterRollback); err != nil || afterRollback != after {
		t.Fatalf("late audit failure left a user behind: before=%d after=%d err=%v", after, afterRollback, err)
	}
}
