//go:build integration

package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testIdentityPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	r := identitypostgres.New(pool)
	first, err := r.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" {
		t.Fatal("missing stable user ID")
	}
	loaded, err := r.GetUser(ctx, first.ID)
	if err != nil || loaded.ID != first.ID || loaded.Status != identity.UserActive {
		t.Fatalf("user round trip failed: %v", err)
	}
	second, err := r.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("users received same ID")
	}
	changed, err := r.UpdateUserStatus(ctx, first.ID, identity.UserSuspended)
	if err != nil || changed.Status != identity.UserSuspended || changed.UpdatedAt.Before(first.UpdatedAt) {
		t.Fatalf("status transition failed: %v", err)
	}
	if _, err := r.CreateUser(ctx, "UNKNOWN"); err == nil {
		t.Fatal("unknown status accepted")
	}
	if _, err := pool.Exec(ctx, "INSERT INTO identity.users (status) VALUES ('UNKNOWN')"); err == nil {
		t.Fatal("database accepted unknown user status")
	}

	email, err := r.CreateUserEmail(ctx, first.ID, " Ada.Example@BTG.org ", true)
	if err != nil {
		t.Fatal(err)
	}
	if email.NormalizedEmail != "ada.example@btg.org" || email.DisplayEmail != "Ada.Example@BTG.org" {
		t.Fatal("email spelling/normalization mismatch")
	}
	found, err := r.GetUserEmailByNormalized(ctx, " ADA.EXAMPLE@btg.ORG ")
	if err != nil || found.ID != email.ID || found.UserID != first.ID {
		t.Fatalf("normalized lookup failed: %v", err)
	}
	primary, err := r.GetPrimaryUserEmail(ctx, first.ID)
	if err != nil || primary.ID != email.ID {
		t.Fatalf("primary email lookup failed: %v", err)
	}
	if _, err := r.CreateUserEmail(ctx, second.ID, "ADA.EXAMPLE@BTG.ORG", true); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("duplicate normalized email was not rejected: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO identity.user_emails (user_id, normalized_email, display_email) VALUES ($1, $2, $3)", second.ID, "ADA.EXAMPLE@BTG.ORG", "ADA.EXAMPLE@BTG.ORG"); err == nil {
		t.Fatal("database accepted non-normalized case variant")
	}
	if _, err := r.CreateUserEmail(ctx, first.ID, "other@example.org", true); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("second primary email accepted: %v", err)
	}
	verified, err := r.MarkUserEmailVerified(ctx, email.ID, time.Now().UTC().Add(time.Second))
	if err != nil || verified.VerifiedAt == nil {
		t.Fatalf("verification timestamp failed: %v", err)
	}

	hash, err := identity.NewPasswordHash("argon2id-test-verifier")
	if err != nil {
		t.Fatal(err)
	}
	credential, err := r.CreateLocalPasswordCredential(ctx, first.ID, hash)
	if err != nil || credential.PasswordHash.Value() != hash.Value() {
		t.Fatalf("credential store failed: %v", err)
	}
	stored, err := r.GetLocalPasswordCredential(ctx, first.ID)
	if err != nil || stored.PasswordHash.Value() != hash.Value() {
		t.Fatalf("credential retrieval failed: %v", err)
	}
	if strings.Contains(fmt.Sprintf("%+v", stored), hash.Value()) {
		t.Fatal("credential formatting revealed password hash")
	}
	userWithoutSecret, err := r.GetUser(ctx, first.ID)
	if err != nil || strings.Contains(fmt.Sprintf("%+v", userWithoutSecret), hash.Value()) {
		t.Fatal("user API revealed password hash")
	}
	newHash, _ := identity.NewPasswordHash("replacement-test-verifier")
	replaced, err := r.ReplaceLocalPasswordHash(ctx, first.ID, newHash)
	if err != nil || replaced.PasswordHash.Value() != newHash.Value() {
		t.Fatalf("credential replacement failed: %v", err)
	}

	assigned, err := r.AssignGlobalRole(ctx, first.ID, identity.RoleAdministrator, nil)
	if err != nil || assigned.GrantedBy != nil {
		t.Fatalf("system role assignment failed: %v", err)
	}
	roles, err := r.ListActiveGlobalRoles(ctx, first.ID)
	if err != nil || len(roles) != 1 || roles[0].Role != identity.RoleAdministrator {
		t.Fatalf("active role query failed: %v", err)
	}
	hasRole, err := r.HasActiveGlobalRole(ctx, first.ID, identity.RoleAdministrator)
	if err != nil || !hasRole {
		t.Fatalf("active role check failed: %v", err)
	}
	if _, err := r.AssignGlobalRole(ctx, first.ID, identity.RoleAdministrator, nil); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("duplicate active role accepted: %v", err)
	}
	revokedRole, err := r.RevokeGlobalRole(ctx, first.ID, identity.RoleAdministrator)
	if err != nil || revokedRole.RevokedAt == nil {
		t.Fatalf("role revocation failed: %v", err)
	}
	hasRole, err = r.HasActiveGlobalRole(ctx, first.ID, identity.RoleAdministrator)
	if err != nil || hasRole {
		t.Fatalf("revoked role remained active: %v", err)
	}
	if _, err := r.AssignGlobalRole(ctx, first.ID, identity.RoleAdministrator, &second.ID); err != nil {
		t.Fatalf("reassignment after revocation failed: %v", err)
	}

	digest1, _ := identity.NewSessionTokenDigest([]byte("11111111111111111111111111111111"))
	digest2, _ := identity.NewSessionTokenDigest([]byte("22222222222222222222222222222222"))
	digest3, _ := identity.NewSessionTokenDigest([]byte("33333333333333333333333333333333"))
	expiry := time.Now().UTC().Add(time.Hour)
	session1, err := r.CreateSession(ctx, first.ID, digest1, expiry)
	if err != nil {
		t.Fatal(err)
	}
	session2, err := r.CreateSession(ctx, first.ID, digest2, expiry)
	if err != nil || session1.ID == session2.ID {
		t.Fatalf("multiple sessions failed: %v", err)
	}
	byDigest, err := r.GetSessionByDigest(ctx, digest1)
	if err != nil || byDigest.ID != session1.ID {
		t.Fatalf("digest lookup failed: %v", err)
	}
	if strings.Contains(fmt.Sprintf("%+v", byDigest), "11111111111111111111111111111111") {
		t.Fatal("session formatting revealed digest")
	}
	if _, err := r.CreateSession(ctx, second.ID, digest1, expiry); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("duplicate token digest accepted: %v", err)
	}
	revokedSession, err := r.RevokeSession(ctx, session1.ID)
	if err != nil || revokedSession.RevokedAt == nil {
		t.Fatalf("session revocation failed: %v", err)
	}
	byDigest, err = r.GetSessionByDigest(ctx, digest1)
	if err != nil || byDigest.RevokedAt == nil {
		t.Fatalf("revoked session not retained: %v", err)
	}
	count, err := r.RevokeUserSessions(ctx, first.ID)
	if err != nil || count != 1 {
		t.Fatalf("revoke all sessions failed: count=%d err=%v", count, err)
	}
	if _, err := r.UpdateSessionLastSeen(ctx, session2.ID); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("revoked session seen timestamp updated: %v", err)
	}

	missingID := "00000000-0000-0000-0000-000000000001"
	if _, err := r.CreateUserEmail(ctx, identity.UserID(missingID), "ghost@example.org", false); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("email foreign key not enforced: %v", err)
	}
	if _, err := r.AssignGlobalRole(ctx, identity.UserID(missingID), identity.RoleAdministrator, nil); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("role foreign key not enforced: %v", err)
	}
	if _, err := r.CreateLocalPasswordCredential(ctx, identity.UserID(missingID), hash); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("credential foreign key not enforced: %v", err)
	}
	if _, err := r.CreateSession(ctx, identity.UserID(missingID), digest3, expiry); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("session foreign key not enforced: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO identity.global_role_assignments (user_id, role) VALUES ($1, 'COURSE_AUTHOR')", first.ID); err == nil {
		t.Fatal("database accepted course role as global role")
	}
	if _, err := pool.Exec(ctx, "INSERT INTO identity.sessions (user_id, token_digest, expires_at) VALUES ($1, $2, $3)", second.ID, []byte("short"), expiry); err == nil {
		t.Fatal("database accepted short session digest")
	}
}
