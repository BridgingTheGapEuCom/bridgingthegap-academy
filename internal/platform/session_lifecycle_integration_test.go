//go:build integration

package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testSessionLifecycle(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	repository := identitypostgres.New(pool)
	user, err := repository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	other, err := repository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	service := identity.NewSessionService(repository, nil, func() time.Time { return now })
	first, err := service.CreateSession(ctx, user.ID)
	if err != nil || first.UserID != user.ID || first.Token.Value() == "" {
		t.Fatalf("session creation failed: %v", err)
	}
	second, err := service.CreateSession(ctx, user.ID)
	if err != nil || second.SessionID == first.SessionID || second.Token.Value() == first.Token.Value() {
		t.Fatalf("independent session creation failed: %v", err)
	}
	otherSession, err := service.CreateSession(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}

	var storedDigest []byte
	var storedUserID string
	var storedLastSeen time.Time
	if err := pool.QueryRow(ctx, "SELECT token_digest, user_id, last_seen_at FROM identity.sessions WHERE id = $1", first.SessionID).Scan(&storedDigest, &storedUserID, &storedLastSeen); err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256([]byte(first.Token.Value()))
	if len(storedDigest) != sha256.Size || !bytes.Equal(storedDigest, wantDigest[:]) || bytes.Equal(storedDigest, []byte(first.Token.Value())) || storedUserID != string(user.ID) {
		t.Fatal("persisted session contains an incorrect digest or owner")
	}
	if _, err := repository.CreateSession(ctx, other.ID, mustSessionDigest(t, storedDigest), now.Add(time.Hour)); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("duplicate digest was not rejected: %v", err)
	}
	resolved, err := service.ResolveSession(ctx, first.Token.Value())
	if err != nil || resolved.SessionID() != first.SessionID || resolved.UserID() != user.ID {
		t.Fatalf("valid session did not resolve: %v", err)
	}
	if err := service.RevokeSession(ctx, first.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeSession(ctx, first.SessionID); err != nil {
		t.Fatalf("repeat revocation failed: %v", err)
	}
	if _, err := service.ResolveSession(ctx, first.Token.Value()); err != identity.ErrInvalidSession {
		t.Fatalf("revoked session remained valid: %v", err)
	}

	count, err := service.RevokeAllSessionsForUser(ctx, user.ID)
	if err != nil || count != 1 {
		t.Fatalf("revoke-all failed: count=%d err=%v", count, err)
	}
	if _, err := service.ResolveSession(ctx, second.Token.Value()); err != identity.ErrInvalidSession {
		t.Fatalf("second user session remained valid: %v", err)
	}
	if _, err := service.ResolveSession(ctx, otherSession.Token.Value()); err != nil {
		t.Fatalf("other user's session was affected: %v", err)
	}

	expiring, err := service.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	now = expiring.ExpiresAt
	if _, err := service.ResolveSession(ctx, expiring.Token.Value()); err != identity.ErrInvalidSession {
		t.Fatalf("session was valid at expiry: %v", err)
	}
	now = time.Now().UTC()
	if _, err := repository.UpdateUserStatus(ctx, user.ID, identity.UserSuspended); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveSession(ctx, expiring.Token.Value()); err != identity.ErrInvalidSession {
		t.Fatalf("suspended user retained session access: %v", err)
	}
	if _, err := service.CreateSession(ctx, user.ID); err != identity.ErrInvalidSession {
		t.Fatalf("suspended user created session: %v", err)
	}

	var beforeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", user.ID).Scan(&beforeCount); err != nil {
		t.Fatal(err)
	}
	unknown := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0}, 32))
	if _, err := service.ResolveSession(ctx, unknown); err != identity.ErrInvalidSession {
		t.Fatalf("unknown token failure was distinguishable: %v", err)
	}
	if _, err := service.ResolveSession(ctx, "malformed"); err != identity.ErrInvalidSession {
		t.Fatalf("malformed token failure was distinguishable: %v", err)
	}
	var afterCount int
	var afterLastSeen time.Time
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM identity.sessions WHERE user_id = $1", user.ID).Scan(&afterCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT last_seen_at FROM identity.sessions WHERE id = $1", first.SessionID).Scan(&afterLastSeen); err != nil {
		t.Fatal(err)
	}
	if afterCount != beforeCount || !afterLastSeen.Equal(storedLastSeen) {
		t.Fatal("invalid lookups changed persisted session state")
	}
}

func mustSessionDigest(t *testing.T, value []byte) identity.SessionTokenDigest {
	t.Helper()
	digest, err := identity.NewSessionTokenDigest(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
