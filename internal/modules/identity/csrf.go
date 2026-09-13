package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"
)

const csrfTokenBytes = 32

var ErrInvalidCSRFToken = errors.New("invalid CSRF token")
var ErrCSRFUnavailable = errors.New("CSRF protection unavailable")

// CSRFToken is a session-bound synchronizer token, not an authentication
// credential. It is stored as sensitive Identity session state and delivered
// only through no-store authentication responses.
type CSRFToken struct{ value string }

func NewCSRFToken(value string) (CSRFToken, error) {
	if len(value) != base64.RawURLEncoding.EncodedLen(csrfTokenBytes) {
		return CSRFToken{}, ErrInvalidCSRFToken
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(decoded) != csrfTokenBytes || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return CSRFToken{}, ErrInvalidCSRFToken
	}
	return CSRFToken{value: value}, nil
}

func (t CSRFToken) Value() string              { return t.value }
func (t CSRFToken) String() string             { return "[REDACTED]" }
func (t CSRFToken) GoString() string           { return "[REDACTED]" }
func (t CSRFToken) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte("[REDACTED]")) }

func randomCSRFToken(source io.Reader) (CSRFToken, error) {
	var data [csrfTokenBytes]byte
	if source == nil {
		return CSRFToken{}, ErrCSRFUnavailable
	}
	if _, err := io.ReadFull(source, data[:]); err != nil {
		return CSRFToken{}, ErrCSRFUnavailable
	}
	return CSRFToken{value: base64.RawURLEncoding.EncodeToString(data[:])}, nil
}

// SessionCSRFRepository exposes only CSRF state for the already resolved
// Identity session. No generic session row or sqlc type crosses this port.
type SessionCSRFRepository interface {
	GetSessionCSRFToken(context.Context, SessionID) (CSRFToken, error)
	InitializeSessionCSRFToken(context.Context, SessionID, CSRFToken) (CSRFToken, error)
}

type SessionCSRFService struct {
	repository SessionCSRFRepository
	now        func() time.Time
	random     io.Reader
}

func NewSessionCSRFService(repository SessionCSRFRepository, now func() time.Time) SessionCSRFService {
	if now == nil {
		now = time.Now
	}
	return SessionCSRFService{repository: repository, now: now, random: rand.Reader}
}

// TokenForSession reads the existing token. A pre-M1.4b session with no token
// gets one once, allowing a current-session GET to upgrade it safely.
func (s SessionCSRFService) TokenForSession(ctx context.Context, current ResolvedSession) (CSRFToken, error) {
	if s.repository == nil || s.now == nil {
		return CSRFToken{}, ErrCSRFUnavailable
	}
	if current.sessionID == "" || current.userID == "" || !s.now().UTC().Before(current.expiresAt) {
		return CSRFToken{}, ErrInvalidSession
	}
	token, err := s.repository.GetSessionCSRFToken(ctx, current.sessionID)
	if errors.Is(err, ErrNotFound) {
		return CSRFToken{}, ErrInvalidSession
	}
	if err != nil {
		return CSRFToken{}, ErrCSRFUnavailable
	}
	if token.Value() != "" {
		if _, err := NewCSRFToken(token.Value()); err != nil {
			return CSRFToken{}, ErrCSRFUnavailable
		}
		return token, nil
	}
	candidate, err := randomCSRFToken(s.random)
	if err != nil {
		return CSRFToken{}, err
	}
	token, err = s.repository.InitializeSessionCSRFToken(ctx, current.sessionID, candidate)
	if errors.Is(err, ErrNotFound) {
		token, err = s.repository.GetSessionCSRFToken(ctx, current.sessionID)
	}
	if errors.Is(err, ErrNotFound) {
		return CSRFToken{}, ErrInvalidSession
	}
	if err != nil || token.Value() == "" {
		return CSRFToken{}, ErrCSRFUnavailable
	}
	if _, err := NewCSRFToken(token.Value()); err != nil {
		return CSRFToken{}, ErrCSRFUnavailable
	}
	return token, nil
}

func (s SessionCSRFService) Validate(ctx context.Context, current ResolvedSession, supplied string) error {
	if _, err := NewCSRFToken(supplied); err != nil {
		return ErrInvalidCSRFToken
	}
	if s.repository == nil || s.now == nil {
		return ErrCSRFUnavailable
	}
	if current.sessionID == "" || current.userID == "" || !s.now().UTC().Before(current.expiresAt) {
		return ErrInvalidSession
	}
	stored, err := s.repository.GetSessionCSRFToken(ctx, current.sessionID)
	if errors.Is(err, ErrNotFound) {
		return ErrInvalidSession
	}
	if err != nil {
		return ErrCSRFUnavailable
	}
	if stored.Value() == "" {
		return ErrInvalidCSRFToken
	}
	if _, err := NewCSRFToken(stored.Value()); err != nil {
		return ErrCSRFUnavailable
	}
	if subtle.ConstantTimeCompare([]byte(stored.Value()), []byte(supplied)) != 1 {
		return ErrInvalidCSRFToken
	}
	return nil
}
