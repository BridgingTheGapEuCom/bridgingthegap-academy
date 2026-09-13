package identity

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sessionTestUser UserID = "00000000-0000-0000-0000-000000000001"
const otherSessionTestUser UserID = "00000000-0000-0000-0000-000000000002"

type sessionClock struct{ at time.Time }

func (c *sessionClock) now() time.Time { return c.at }

type sessionRepositoryFake struct {
	users          map[UserID]User
	sessions       map[string]Session
	now            func() time.Time
	getUserErr     error
	createErr      error
	lookupErr      error
	revokeErr      error
	revokeAllErr   error
	lookupCount    int
	createCount    int
	createdDigests []SessionTokenDigest
}

func newSessionFixture() (*sessionRepositoryFake, *sessionClock, SessionService) {
	clock := &sessionClock{at: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)}
	repository := &sessionRepositoryFake{
		users: map[UserID]User{
			sessionTestUser:      {ID: sessionTestUser, Status: UserActive},
			otherSessionTestUser: {ID: otherSessionTestUser, Status: UserActive},
		},
		sessions: make(map[string]Session), now: clock.now,
	}
	service := NewSessionService(repository, nil, clock.now)
	return repository, clock, service
}

func (f *sessionRepositoryFake) GetUser(_ context.Context, id UserID) (User, error) {
	if f.getUserErr != nil {
		return User{}, f.getUserErr
	}
	user, ok := f.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}
func (f *sessionRepositoryFake) CreateSession(_ context.Context, userID UserID, digest SessionTokenDigest, expires time.Time) (Session, error) {
	if f.createErr != nil {
		return Session{}, f.createErr
	}
	f.createCount++
	f.createdDigests = append(f.createdDigests, digest)
	session := Session{ID: SessionID(fmt.Sprintf("session-%d", f.createCount)), UserID: userID, TokenDigest: digest, CreatedAt: f.now(), LastSeenAt: f.now(), ExpiresAt: expires}
	f.sessions[string(digest.Bytes())] = session
	return session, nil
}
func (f *sessionRepositoryFake) GetSessionByDigest(_ context.Context, digest SessionTokenDigest) (Session, error) {
	f.lookupCount++
	if f.lookupErr != nil {
		return Session{}, f.lookupErr
	}
	session, ok := f.sessions[string(digest.Bytes())]
	if !ok {
		return Session{}, ErrNotFound
	}
	return session, nil
}
func (f *sessionRepositoryFake) RevokeSession(_ context.Context, id SessionID) (Session, error) {
	if f.revokeErr != nil {
		return Session{}, f.revokeErr
	}
	for key, session := range f.sessions {
		if session.ID == id && session.RevokedAt == nil {
			at := f.now()
			session.RevokedAt = &at
			f.sessions[key] = session
			return session, nil
		}
	}
	return Session{}, ErrNotFound
}
func (f *sessionRepositoryFake) RevokeUserSessions(_ context.Context, userID UserID) (int64, error) {
	if f.revokeAllErr != nil {
		return 0, f.revokeAllErr
	}
	var count int64
	for key, session := range f.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			at := f.now()
			session.RevokedAt = &at
			f.sessions[key] = session
			count++
		}
	}
	return count, nil
}

type sessionObserverFake struct{ events []SessionSecurityEvent }

func (f *sessionObserverFake) ObserveSessionSecurity(_ context.Context, event SessionSecurityEvent) {
	f.events = append(f.events, event)
}

func TestSessionTokenGenerationAndDigest(t *testing.T) {
	repository, clock, service := newSessionFixture()
	if service.random != rand.Reader {
		t.Fatal("production session service does not use crypto/rand")
	}
	// A deterministic source proves each creation consumes fresh 32-byte input
	// without making the test depend on a statistical comparison.
	service.random = bytes.NewReader(append(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)...))
	ctx := context.Background()
	first, err := service.CreateSession(ctx, sessionTestUser)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSession(ctx, sessionTestUser)
	if err != nil {
		t.Fatal(err)
	}
	if first.Token.Value() == second.Token.Value() || first.SessionID == second.SessionID {
		t.Fatal("session creation reused a token or row identity")
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(first.Token.Value())
	if err != nil || len(decoded) != 32 || len(first.Token.Value()) != 43 {
		t.Fatal("token is not 32 random bytes in canonical URL-safe encoding")
	}
	sum := sha256.Sum256([]byte(first.Token.Value()))
	if len(repository.createdDigests) != 2 || len(repository.createdDigests[0].Bytes()) != sha256.Size || !bytes.Equal(repository.createdDigests[0].Bytes(), sum[:]) {
		t.Fatal("repository did not receive only the SHA-256 token digest")
	}
	if first.UserID != sessionTestUser || !first.IssuedAt.Equal(clock.at) || !first.ExpiresAt.Equal(clock.at.Add(DefaultAbsoluteSessionLifetime)) {
		t.Fatal("created session metadata or absolute lifetime is wrong")
	}
	if strings.Contains(fmt.Sprintf("%+v", first), first.Token.Value()) || strings.Contains(fmt.Sprintf("%#v", first), first.Token.Value()) {
		t.Fatal("formatting leaked the raw session token")
	}
}

func TestSessionCreationRequiresActiveUserAndOperationalFailuresStayDistinct(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		setup func(*sessionRepositoryFake)
		want  error
	}{
		{"suspended", func(r *sessionRepositoryFake) {
			r.users[sessionTestUser] = User{ID: sessionTestUser, Status: UserSuspended}
		}, ErrInvalidSession},
		{"missing", func(r *sessionRepositoryFake) { delete(r.users, sessionTestUser) }, ErrInvalidSession},
		{"get user failed", func(r *sessionRepositoryFake) { r.getUserErr = errors.New("private database detail") }, ErrSessionUnavailable},
		{"insert failed", func(r *sessionRepositoryFake) { r.createErr = errors.New("private database detail") }, ErrSessionUnavailable},
		{"corrupt user", func(r *sessionRepositoryFake) {
			r.users[sessionTestUser] = User{ID: sessionTestUser, Status: "UNKNOWN"}
		}, ErrCorruptSession},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repository, _, service := newSessionFixture()
			scenario.setup(repository)
			result, err := service.CreateSession(context.Background(), sessionTestUser)
			if err != scenario.want || result != (CreatedSession{}) || strings.Contains(err.Error(), "private database detail") {
				t.Fatalf("incorrect creation failure: %v", err)
			}
			if scenario.name != "insert failed" && repository.createCount != 0 {
				t.Fatal("created a row for an unavailable user")
			}
		})
	}
	_, _, service := newSessionFixture()
	service.random = bytes.NewReader(nil)
	if _, err := service.CreateSession(context.Background(), sessionTestUser); err != ErrSessionUnavailable {
		t.Fatalf("random-source failure did not fail closed: %v", err)
	}
}

func TestSessionResolutionUsesClockAndOneGenericInvalidError(t *testing.T) {
	repository, clock, service := newSessionFixture()
	ctx := context.Background()
	created, err := service.CreateSession(ctx, sessionTestUser)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveSession(ctx, created.Token.Value())
	if err != nil || resolved.UserID != sessionTestUser || resolved.SessionID != created.SessionID || !resolved.ExpiresAt.Equal(created.ExpiresAt) {
		t.Fatalf("valid session did not resolve: %v", err)
	}
	if _, ok := reflect.TypeOf(resolved).FieldByName("TokenDigest"); ok {
		t.Fatal("resolved result exposes digest")
	}
	if _, ok := reflect.TypeOf(resolved).FieldByName("Token"); ok {
		t.Fatal("resolved result exposes bearer token")
	}
	if strings.Contains(fmt.Sprintf("%+v", resolved), created.Token.Value()) {
		t.Fatal("resolved result exposed token")
	}

	for _, bad := range []string{"", "not-a-token", strings.Repeat("a", 44), base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0}, 32))} {
		before := repository.lookupCount
		_, err := service.ResolveSession(ctx, bad)
		if err != ErrInvalidSession {
			t.Fatalf("malformed or unknown token was distinguishable: %v", err)
		}
		if bad == "" || bad == "not-a-token" || len(bad) == 44 {
			if repository.lookupCount != before {
				t.Fatal("malformed token reached persistence")
			}
		}
	}

	clock.at = created.ExpiresAt
	if _, err := service.ResolveSession(ctx, created.Token.Value()); err != ErrInvalidSession {
		t.Fatalf("expiry boundary accepted: %v", err)
	}
	clock.at = created.IssuedAt
	repository.users[sessionTestUser] = User{ID: sessionTestUser, Status: UserSuspended}
	if _, err := service.ResolveSession(ctx, created.Token.Value()); err != ErrInvalidSession {
		t.Fatalf("suspended owner retained access: %v", err)
	}
	repository.users[sessionTestUser] = User{ID: sessionTestUser, Status: UserActive}
	if err := service.RevokeSession(ctx, created.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveSession(ctx, created.Token.Value()); err != ErrInvalidSession {
		t.Fatalf("revoked session resolved: %v", err)
	}
}

func TestSessionResolutionOperationalAndCorruptStateFailures(t *testing.T) {
	repository, _, service := newSessionFixture()
	ctx := context.Background()
	created, err := service.CreateSession(ctx, sessionTestUser)
	if err != nil {
		t.Fatal(err)
	}
	repository.lookupErr = errors.New("private database detail")
	if _, err := service.ResolveSession(ctx, created.Token.Value()); err != ErrSessionUnavailable || strings.Contains(err.Error(), "private database detail") {
		t.Fatalf("query error was not classified safely: %v", err)
	}
	repository.lookupErr = nil
	repository.getUserErr = errors.New("private database detail")
	if _, err := service.ResolveSession(ctx, created.Token.Value()); err != ErrSessionUnavailable {
		t.Fatalf("owner query error was misclassified: %v", err)
	}
	repository.getUserErr = nil
	observer := &sessionObserverFake{}
	service.observer = observer
	key := string(repository.createdDigests[0].Bytes())
	broken := repository.sessions[key]
	broken.TokenDigest, _ = NewSessionTokenDigest(bytes.Repeat([]byte{1}, 32))
	repository.sessions[key] = broken
	if _, err := service.ResolveSession(ctx, created.Token.Value()); err != ErrCorruptSession || len(observer.events) != 1 || observer.events[0].Outcome != SessionPersistenceCorrupt {
		t.Fatalf("corrupt row was not signaled safely: %v", err)
	}
}

func TestSessionRevocationIsRepeatableAndScopedToUser(t *testing.T) {
	repository, _, service := newSessionFixture()
	observer := &sessionObserverFake{}
	service.observer = observer
	ctx := context.Background()
	first, _ := service.CreateSession(ctx, sessionTestUser)
	second, _ := service.CreateSession(ctx, sessionTestUser)
	other, _ := service.CreateSession(ctx, otherSessionTestUser)
	if err := service.RevokeSession(ctx, first.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeSession(ctx, first.SessionID); err != nil {
		t.Fatalf("repeated revoke failed: %v", err)
	}
	count, err := service.RevokeAllSessionsForUser(ctx, sessionTestUser)
	if err != nil || count != 1 {
		t.Fatalf("revoke-all did not affect only remaining active session: count=%d err=%v", count, err)
	}
	if _, err := service.ResolveSession(ctx, second.Token.Value()); err != ErrInvalidSession {
		t.Fatalf("user session remained active: %v", err)
	}
	if _, err := service.ResolveSession(ctx, other.Token.Value()); err != nil {
		t.Fatalf("other user session was revoked: %v", err)
	}
	count, err = service.RevokeAllSessionsForUser(ctx, sessionTestUser)
	if err != nil || count != 0 {
		t.Fatalf("repeated revoke-all failed: count=%d err=%v", count, err)
	}
	if len(observer.events) != 3 || observer.events[0].Outcome != SessionRevoked || observer.events[1].Outcome != UserSessionsRevoked || observer.events[1].Count != 1 {
		t.Fatalf("revocation signals are wrong: %+v", observer.events)
	}
	for _, event := range observer.events {
		if strings.Contains(fmt.Sprintf("%+v", event), first.Token.Value()) || strings.Contains(fmt.Sprintf("%+v", event), second.Token.Value()) {
			t.Fatal("security signal exposed a bearer token")
		}
	}
	repository.revokeErr = errors.New("private database detail")
	if err := service.RevokeSession(ctx, other.SessionID); err != ErrSessionUnavailable {
		t.Fatalf("revoke storage error was misclassified: %v", err)
	}
	repository.revokeAllErr = errors.New("private database detail")
	if _, err := service.RevokeAllSessionsForUser(ctx, otherSessionTestUser); err != ErrSessionUnavailable {
		t.Fatalf("revoke-all storage error was misclassified: %v", err)
	}
}
