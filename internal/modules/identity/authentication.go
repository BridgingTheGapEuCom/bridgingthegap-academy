package identity

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrAuthenticationUnavailable = errors.New("password authentication unavailable")
var ErrCorruptPasswordCredential = errors.New("stored password credential is invalid")

type AuthenticationMethod string

const AuthenticationMethodLocalPassword AuthenticationMethod = "LOCAL_PASSWORD"

// AuthenticatedIdentity contains only the facts needed by later session creation.
type AuthenticatedIdentity struct {
	UserID          UserID
	Method          AuthenticationMethod
	AuthenticatedAt time.Time
}

// PasswordAuthenticationRepository is the read-only slice of existing Identity
// repository contracts required for local password authentication.
type PasswordAuthenticationRepository interface {
	GetUserEmailByNormalized(context.Context, string) (UserEmail, error)
	GetUser(context.Context, UserID) (User, error)
	GetLocalPasswordCredential(context.Context, UserID) (LocalPasswordCredential, error)
}

type PasswordVerifier interface {
	VerifyPassword(PasswordHash, []byte) (bool, error)
}

type PasswordAuthenticationOutcome string

const (
	PasswordAuthenticationSucceeded         PasswordAuthenticationOutcome = "SUCCEEDED"
	PasswordAuthenticationUnknownAccount    PasswordAuthenticationOutcome = "UNKNOWN_ACCOUNT"
	PasswordAuthenticationWrongPassword     PasswordAuthenticationOutcome = "WRONG_PASSWORD"
	PasswordAuthenticationMissingCredential PasswordAuthenticationOutcome = "MISSING_CREDENTIAL"
	PasswordAuthenticationSuspendedAccount  PasswordAuthenticationOutcome = "SUSPENDED_ACCOUNT"
	PasswordAuthenticationCorruptCredential PasswordAuthenticationOutcome = "CORRUPT_CREDENTIAL"
	PasswordAuthenticationRepositoryFailure PasswordAuthenticationOutcome = "REPOSITORY_FAILURE"
	PasswordAuthenticationVerifierFailure   PasswordAuthenticationOutcome = "VERIFIER_FAILURE"
	PasswordAuthenticationInvalidInput      PasswordAuthenticationOutcome = "INVALID_INPUT"
)

// PasswordAuthenticationAttempt is an internal security signal. It deliberately
// excludes the submitted email, password, and credential. UserID is empty when
// no account was resolved; it must never be filled with a synthetic identifier.
type PasswordAuthenticationAttempt struct {
	Outcome PasswordAuthenticationOutcome
	UserID  UserID
}

type PasswordAuthenticationObserver interface {
	// Final login audit belongs with later session creation; this signal lets
	// the caller observe attempts without recording a completed login early.
	ObservePasswordAuthentication(context.Context, PasswordAuthenticationAttempt)
}

type PasswordAuthenticator struct {
	repository PasswordAuthenticationRepository
	verifier   PasswordVerifier
	observer   PasswordAuthenticationObserver
	now        func() time.Time
}

// This valid, precomputed PHC hash is used only to spend Argon2id work when no
// account or local credential exists. Its random source password is discarded.
// It uses the current interactive defaults; no hash is generated per attempt.
const dummyPasswordPHC = "$argon2id$v=19$m=65536,t=3,p=1$a4+TPrXlE4WU0RxnNK0G2g$4aBD6O0b55u4vrVuPhvGWD+sk1q4jER8yqkRuh6wVj4"

var dummyPasswordHash = PasswordHash{value: dummyPasswordPHC}

func NewPasswordAuthenticator(repository PasswordAuthenticationRepository, verifier PasswordVerifier, observer PasswordAuthenticationObserver, now func() time.Time) PasswordAuthenticator {
	if now == nil {
		now = time.Now
	}
	return PasswordAuthenticator{repository: repository, verifier: verifier, observer: observer, now: now}
}

// AuthenticatePassword checks local credentials and does not create a session.
// The caller's password byte slice is cleared before this method returns.
func (a PasswordAuthenticator) AuthenticatePassword(ctx context.Context, email string, password []byte) (AuthenticatedIdentity, error) {
	defer clear(password)
	if a.repository == nil || a.verifier == nil {
		return AuthenticatedIdentity{}, ErrAuthenticationUnavailable
	}
	// Oversized input is rejected before any account lookup, so this early path
	// does not reveal whether an email has a matching account.
	if len(password) > MaximumPasswordBytes {
		a.observe(ctx, PasswordAuthenticationInvalidInput, "")
		return AuthenticatedIdentity{}, ErrInvalidCredentials
	}
	normalized, err := NormalizeEmail(email)
	if err != nil {
		a.observe(ctx, PasswordAuthenticationInvalidInput, "")
		return AuthenticatedIdentity{}, ErrInvalidCredentials
	}
	userEmail, err := a.repository.GetUserEmailByNormalized(ctx, normalized)
	if errors.Is(err, ErrNotFound) {
		return a.dummyFailure(ctx, password, PasswordAuthenticationUnknownAccount, "")
	}
	if err != nil {
		return a.storageFailure(ctx, "")
	}
	user, err := a.repository.GetUser(ctx, userEmail.UserID)
	if err != nil {
		return a.storageFailure(ctx, userEmail.UserID)
	}
	credential, err := a.repository.GetLocalPasswordCredential(ctx, user.ID)
	if errors.Is(err, ErrNotFound) {
		return a.dummyFailure(ctx, password, PasswordAuthenticationMissingCredential, user.ID)
	}
	if err != nil {
		return a.storageFailure(ctx, user.ID)
	}
	verified, err := a.verifier.VerifyPassword(credential.PasswordHash, password)
	if errors.Is(err, ErrMalformedPasswordHash) {
		a.observe(ctx, PasswordAuthenticationCorruptCredential, user.ID)
		return AuthenticatedIdentity{}, ErrCorruptPasswordCredential
	}
	if err != nil {
		return a.verifierFailure(ctx, user.ID)
	}
	if user.Status != UserActive {
		a.observe(ctx, PasswordAuthenticationSuspendedAccount, user.ID)
		return AuthenticatedIdentity{}, ErrInvalidCredentials
	}
	if !verified {
		a.observe(ctx, PasswordAuthenticationWrongPassword, user.ID)
		return AuthenticatedIdentity{}, ErrInvalidCredentials
	}
	a.observe(ctx, PasswordAuthenticationSucceeded, user.ID)
	return AuthenticatedIdentity{UserID: user.ID, Method: AuthenticationMethodLocalPassword, AuthenticatedAt: a.now().UTC()}, nil
}

func (a PasswordAuthenticator) dummyFailure(ctx context.Context, password []byte, outcome PasswordAuthenticationOutcome, userID UserID) (AuthenticatedIdentity, error) {
	if _, err := a.verifier.VerifyPassword(dummyPasswordHash, password); err != nil {
		return a.verifierFailure(ctx, userID)
	}
	a.observe(ctx, outcome, userID)
	return AuthenticatedIdentity{}, ErrInvalidCredentials
}

func (a PasswordAuthenticator) storageFailure(ctx context.Context, userID UserID) (AuthenticatedIdentity, error) {
	a.observe(ctx, PasswordAuthenticationRepositoryFailure, userID)
	return AuthenticatedIdentity{}, ErrAuthenticationUnavailable
}

func (a PasswordAuthenticator) verifierFailure(ctx context.Context, userID UserID) (AuthenticatedIdentity, error) {
	a.observe(ctx, PasswordAuthenticationVerifierFailure, userID)
	return AuthenticatedIdentity{}, ErrAuthenticationUnavailable
}

func (a PasswordAuthenticator) observe(ctx context.Context, outcome PasswordAuthenticationOutcome, userID UserID) {
	if a.observer != nil {
		a.observer.ObservePasswordAuthentication(ctx, PasswordAuthenticationAttempt{Outcome: outcome, UserID: userID})
	}
}
