package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	// A session has a fixed absolute expiry. Idle expiry and last-seen refresh
	// are deferred until a policy for their write frequency is defined.
	DefaultAbsoluteSessionLifetime = 7 * 24 * time.Hour
	sessionTokenBytes              = 32 // 256 bits from crypto/rand.
)

var ErrInvalidSession = errors.New("invalid session")
var ErrSessionUnavailable = errors.New("session service unavailable")
var ErrCorruptSession = errors.New("stored session is invalid")

// RawSessionToken is a bearer secret. Value is only for the caller that will
// transport it; formatting always redacts it, including inside other structs.
type RawSessionToken struct{ value string }

func (t RawSessionToken) Value() string                 { return t.value }
func (t RawSessionToken) String() string                { return "[REDACTED]" }
func (t RawSessionToken) GoString() string              { return "[REDACTED]" }
func (t RawSessionToken) Format(s fmt.State, verb rune) { _, _ = s.Write([]byte("[REDACTED]")) }

// CreatedSession is returned only to the caller creating the session. No
// persistence model or resolved-session result contains the bearer token.
type CreatedSession struct {
	Token     RawSessionToken
	SessionID SessionID
	UserID    UserID
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type ResolvedSession struct {
	SessionID SessionID
	UserID    UserID
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// SessionLifecycleRepository is the focused slice of Identity persistence
// needed by the transport-independent session service.
type SessionLifecycleRepository interface {
	GetUser(context.Context, UserID) (User, error)
	CreateSession(context.Context, UserID, SessionTokenDigest, time.Time) (Session, error)
	GetSessionByDigest(context.Context, SessionTokenDigest) (Session, error)
	GetSessionByID(context.Context, SessionID) (Session, error)
	RevokeSession(context.Context, SessionID) (Session, error)
	RevokeUserSessions(context.Context, UserID) (int64, error)
}

type SessionSecurityOutcome string

const (
	SessionRevoked            SessionSecurityOutcome = "SESSION_REVOKED"
	UserSessionsRevoked       SessionSecurityOutcome = "USER_SESSIONS_REVOKED"
	SessionPersistenceCorrupt SessionSecurityOutcome = "SESSION_PERSISTENCE_CORRUPT"
)

// SessionSecurityEvent contains only stable internal IDs. It is a signal for
// future security/audit orchestration, not a completed-login audit record.
type SessionSecurityEvent struct {
	Outcome   SessionSecurityOutcome
	UserID    UserID
	SessionID SessionID
	Count     int64
}

type SessionSecurityObserver interface {
	ObserveSessionSecurity(context.Context, SessionSecurityEvent)
}

type SessionService struct {
	repository SessionLifecycleRepository
	observer   SessionSecurityObserver
	now        func() time.Time
	random     io.Reader
}

func NewSessionService(repository SessionLifecycleRepository, observer SessionSecurityObserver, now func() time.Time) SessionService {
	if now == nil {
		now = time.Now
	}
	return SessionService{repository: repository, observer: observer, now: now, random: rand.Reader}
}

// CreateSession always generates a fresh opaque token. It never accepts a
// caller-selected token, session ID, or authorization roles.
func (s SessionService) CreateSession(ctx context.Context, userID UserID) (CreatedSession, error) {
	if s.repository == nil || s.random == nil || s.now == nil {
		return CreatedSession{}, ErrSessionUnavailable
	}
	user, err := s.repository.GetUser(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return CreatedSession{}, ErrInvalidSession
	}
	if err != nil {
		return CreatedSession{}, ErrSessionUnavailable
	}
	if user.ID != userID || user.ID == "" {
		s.observe(ctx, SessionPersistenceCorrupt, userID, "", 0)
		return CreatedSession{}, ErrCorruptSession
	}
	if user.Status == UserSuspended {
		return CreatedSession{}, ErrInvalidSession
	}
	if user.Status != UserActive {
		s.observe(ctx, SessionPersistenceCorrupt, userID, "", 0)
		return CreatedSession{}, ErrCorruptSession
	}

	bytes := make([]byte, sessionTokenBytes)
	if _, err := io.ReadFull(s.random, bytes); err != nil {
		return CreatedSession{}, ErrSessionUnavailable
	}
	token := RawSessionToken{value: base64.RawURLEncoding.EncodeToString(bytes)}
	sum := sha256.Sum256([]byte(token.value))
	digest, _ := NewSessionTokenDigest(sum[:]) // SHA-256 always returns 32 bytes.
	session, err := s.repository.CreateSession(ctx, userID, digest, s.now().UTC().Add(DefaultAbsoluteSessionLifetime))
	if err != nil {
		return CreatedSession{}, ErrSessionUnavailable
	}
	if !validPersistedSession(session, digest) || session.UserID != userID || session.RevokedAt != nil {
		s.observe(ctx, SessionPersistenceCorrupt, userID, session.ID, 0)
		return CreatedSession{}, ErrCorruptSession
	}
	return CreatedSession{Token: token, SessionID: session.ID, UserID: userID, IssuedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt}, nil
}

func (s SessionService) ResolveSession(ctx context.Context, rawToken string) (ResolvedSession, error) {
	if s.repository == nil || s.now == nil {
		return ResolvedSession{}, ErrSessionUnavailable
	}
	if len(rawToken) != base64.RawURLEncoding.EncodedLen(sessionTokenBytes) {
		return ResolvedSession{}, ErrInvalidSession
	}
	bytes, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(bytes) != sessionTokenBytes || base64.RawURLEncoding.EncodeToString(bytes) != rawToken {
		return ResolvedSession{}, ErrInvalidSession
	}
	sum := sha256.Sum256([]byte(rawToken))
	digest, _ := NewSessionTokenDigest(sum[:])
	session, err := s.repository.GetSessionByDigest(ctx, digest)
	if errors.Is(err, ErrNotFound) {
		return ResolvedSession{}, ErrInvalidSession
	}
	if err != nil {
		return ResolvedSession{}, ErrSessionUnavailable
	}
	if !validPersistedSession(session, digest) {
		s.observe(ctx, SessionPersistenceCorrupt, session.UserID, session.ID, 0)
		return ResolvedSession{}, ErrCorruptSession
	}
	if session.RevokedAt != nil || !s.now().UTC().Before(session.ExpiresAt) {
		return ResolvedSession{}, ErrInvalidSession
	}
	user, err := s.repository.GetUser(ctx, session.UserID)
	if errors.Is(err, ErrNotFound) {
		s.observe(ctx, SessionPersistenceCorrupt, session.UserID, session.ID, 0)
		return ResolvedSession{}, ErrCorruptSession
	}
	if err != nil {
		return ResolvedSession{}, ErrSessionUnavailable
	}
	if user.ID != session.UserID || user.Status != UserActive && user.Status != UserSuspended {
		s.observe(ctx, SessionPersistenceCorrupt, session.UserID, session.ID, 0)
		return ResolvedSession{}, ErrCorruptSession
	}
	if user.Status == UserSuspended {
		return ResolvedSession{}, ErrInvalidSession
	}
	return ResolvedSession{SessionID: session.ID, UserID: session.UserID, IssuedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt}, nil
}

// RevokeSession is repeatable: an already-revoked or absent session is a no-op.
// The caller must authorize the session identity before invoking this method.
func (s SessionService) RevokeSession(ctx context.Context, sessionID SessionID) error {
	return s.revokeSession(ctx, sessionID)
}

// RevokeResolvedSession checks that the row being revoked belongs to the
// trusted current-session context before changing it.
func (s SessionService) RevokeResolvedSession(ctx context.Context, current ResolvedSession) error {
	if current.UserID == "" || current.SessionID == "" {
		return ErrInvalidSession
	}
	if s.repository == nil {
		return ErrSessionUnavailable
	}
	session, err := s.repository.GetSessionByID(ctx, current.SessionID)
	if errors.Is(err, ErrNotFound) {
		return ErrInvalidSession
	}
	if err != nil {
		return ErrSessionUnavailable
	}
	if session.ID != current.SessionID || session.UserID != current.UserID {
		return ErrInvalidSession
	}
	return s.revokeSession(ctx, current.SessionID)
}

func (s SessionService) revokeSession(ctx context.Context, sessionID SessionID) error {
	if s.repository == nil {
		return ErrSessionUnavailable
	}
	if sessionID == "" {
		return ErrInvalidSession
	}
	session, err := s.repository.RevokeSession(ctx, sessionID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return ErrSessionUnavailable
	}
	if session.ID != sessionID || session.UserID == "" || session.RevokedAt == nil {
		s.observe(ctx, SessionPersistenceCorrupt, session.UserID, sessionID, 0)
		return ErrCorruptSession
	}
	s.observe(ctx, SessionRevoked, session.UserID, sessionID, 1)
	return nil
}

// RevokeAllSessionsForUser only updates Identity-owned session rows.
// Authorization of this operation belongs to a later caller/orchestrator.
func (s SessionService) RevokeAllSessionsForUser(ctx context.Context, userID UserID) (int64, error) {
	if s.repository == nil {
		return 0, ErrSessionUnavailable
	}
	if userID == "" {
		return 0, ErrInvalidSession
	}
	count, err := s.repository.RevokeUserSessions(ctx, userID)
	if err != nil {
		return 0, ErrSessionUnavailable
	}
	if count < 0 {
		s.observe(ctx, SessionPersistenceCorrupt, userID, "", 0)
		return 0, ErrCorruptSession
	}
	s.observe(ctx, UserSessionsRevoked, userID, "", count)
	return count, nil
}

func validPersistedSession(session Session, digest SessionTokenDigest) bool {
	return session.ID != "" && session.UserID != "" &&
		subtle.ConstantTimeCompare(session.TokenDigest.Bytes(), digest.Bytes()) == 1 &&
		!session.CreatedAt.IsZero() && !session.LastSeenAt.Before(session.CreatedAt) &&
		session.ExpiresAt.After(session.CreatedAt) &&
		(session.RevokedAt == nil || !session.RevokedAt.Before(session.CreatedAt))
}

func (s SessionService) observe(ctx context.Context, outcome SessionSecurityOutcome, userID UserID, sessionID SessionID, count int64) {
	if s.observer != nil {
		s.observer.ObserveSessionSecurity(ctx, SessionSecurityEvent{Outcome: outcome, UserID: userID, SessionID: sessionID, Count: count})
	}
}
