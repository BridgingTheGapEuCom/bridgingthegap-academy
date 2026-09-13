package identity

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestSessionCSRFTokenIsFreshSessionBoundAndRedacted(t *testing.T) {
	repository, clock, sessions := newSessionFixture()
	first, err := sessions.CreateSession(context.Background(), sessionTestUser)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sessions.CreateSession(context.Background(), sessionTestUser)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(first.CSRFToken.Value())
	if err != nil || len(decoded) != csrfTokenBytes || first.CSRFToken.Value() == second.CSRFToken.Value() || first.CSRFToken.Value() == first.Token.Value() {
		t.Fatal("CSRF token lacks independent 256-bit session state")
	}
	if strings.Contains(fmt.Sprintf("%+v", first), first.CSRFToken.Value()) || strings.Contains(fmt.Sprintf("%+v", repository.sessions), first.CSRFToken.Value()) {
		t.Fatal("unrelated session diagnostic exposed CSRF token")
	}
	firstContext, err := sessions.ResolveSession(context.Background(), first.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	secondContext, err := sessions.ResolveSession(context.Background(), second.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	service := NewSessionCSRFService(repository, clock.now)
	if token, err := service.TokenForSession(context.Background(), firstContext); err != nil || token.Value() != first.CSRFToken.Value() {
		t.Fatal("session CSRF token was not retrieved")
	}
	if err := service.Validate(context.Background(), firstContext, first.CSRFToken.Value()); err != nil {
		t.Fatal(err)
	}
	if err := service.Validate(context.Background(), secondContext, first.CSRFToken.Value()); err != ErrInvalidCSRFToken {
		t.Fatal("CSRF token crossed session boundary")
	}
	before := repository.csrfReads
	for _, invalid := range []string{"", "malformed", strings.Repeat("A", 10_000)} {
		if err := service.Validate(context.Background(), firstContext, invalid); err != ErrInvalidCSRFToken || repository.csrfReads != before {
			t.Fatal("malformed CSRF token reached persistence")
		}
	}
	clock.at = first.ExpiresAt
	if err := service.Validate(context.Background(), firstContext, first.CSRFToken.Value()); err != ErrInvalidSession {
		t.Fatal("expired session accepted CSRF token")
	}
}

func TestLegacySessionGetsOneCSRFToken(t *testing.T) {
	repository, clock, sessions := newSessionFixture()
	created, err := sessions.CreateSession(context.Background(), sessionTestUser)
	if err != nil {
		t.Fatal(err)
	}
	current, err := sessions.ResolveSession(context.Background(), created.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	repository.csrfTokens[created.SessionID] = CSRFToken{} // pre-M1.4b row
	service := NewSessionCSRFService(repository, clock.now)
	first, err := service.TokenForSession(context.Background(), current)
	if err != nil || first.Value() == "" {
		t.Fatal("legacy session could not obtain CSRF state")
	}
	second, err := service.TokenForSession(context.Background(), current)
	if err != nil || first.Value() != second.Value() {
		t.Fatal("legacy session CSRF state was rotated unexpectedly")
	}
	if err := sessions.RevokeSession(context.Background(), created.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := service.Validate(context.Background(), current, first.Value()); err != ErrInvalidSession {
		t.Fatal("revoked session retained CSRF validation")
	}
}
