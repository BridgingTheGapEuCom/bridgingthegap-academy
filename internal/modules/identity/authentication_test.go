package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const authTestUserID UserID = "00000000-0000-0000-0000-000000000001"

type authenticationRepositoryFake struct {
	email         UserEmail
	user          User
	credential    LocalPasswordCredential
	emailErr      error
	userErr       error
	credentialErr error
	lookupEmail   string
}

func (f *authenticationRepositoryFake) GetUserEmailByNormalized(_ context.Context, email string) (UserEmail, error) {
	f.lookupEmail = email
	return f.email, f.emailErr
}
func (f *authenticationRepositoryFake) GetUser(context.Context, UserID) (User, error) {
	return f.user, f.userErr
}
func (f *authenticationRepositoryFake) GetLocalPasswordCredential(context.Context, UserID) (LocalPasswordCredential, error) {
	return f.credential, f.credentialErr
}

type authenticationObserverFake struct {
	attempts []PasswordAuthenticationAttempt
}

func (f *authenticationObserverFake) ObservePasswordAuthentication(_ context.Context, attempt PasswordAuthenticationAttempt) {
	f.attempts = append(f.attempts, attempt)
}

type passwordVerifierSpy struct {
	calledWith []PasswordHash
	valid      bool
	err        error
}

func (v *passwordVerifierSpy) VerifyPassword(hash PasswordHash, _ []byte) (bool, error) {
	v.calledWith = append(v.calledWith, hash)
	return v.valid, v.err
}

func realAuthenticationFixture(t *testing.T) (*authenticationRepositoryFake, PasswordVerifier) {
	t.Helper()
	hasher := testPasswordHasher(t)
	hash, err := hasher.HashPassword([]byte("correct horse battery staple"))
	if err != nil {
		t.Fatal(err)
	}
	return &authenticationRepositoryFake{
		email:      UserEmail{UserID: authTestUserID, NormalizedEmail: "admin@example.com"},
		user:       User{ID: authTestUserID, Status: UserActive},
		credential: LocalPasswordCredential{UserID: authTestUserID, PasswordHash: hash},
	}, hasher
}

func TestPasswordAuthenticatorActiveAccountSucceedsWithMinimalResult(t *testing.T) {
	repository, verifier := realAuthenticationFixture(t)
	observer := &authenticationObserverFake{}
	at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	service := NewPasswordAuthenticator(repository, verifier, observer, func() time.Time { return at })
	password := []byte("correct horse battery staple")
	result, err := service.AuthenticatePassword(context.Background(), " ADMIN@Example.com ", password)
	if err != nil || result.UserID != authTestUserID || result.Method != AuthenticationMethodLocalPassword || !result.AuthenticatedAt.Equal(at) {
		t.Fatalf("active account did not authenticate: %+v, %v", result, err)
	}
	if repository.lookupEmail != "admin@example.com" {
		t.Fatalf("email was not normalized before lookup: %q", repository.lookupEmail)
	}
	if strings.Contains(fmt.Sprintf("%+v", result), repository.credential.PasswordHash.Value()) || strings.Contains(fmt.Sprintf("%+v", result), "admin@example.com") {
		t.Fatal("authentication result exposed credential or email data")
	}
	if len(observer.attempts) != 1 || observer.attempts[0].Outcome != PasswordAuthenticationSucceeded || observer.attempts[0].UserID != authTestUserID {
		t.Fatalf("success signal missing: %+v", observer.attempts)
	}
	if strings.Trim(string(password), "\x00") != "" {
		t.Fatal("plaintext password was not cleared")
	}
}

func TestPasswordAuthenticatorOrdinaryFailuresShareOneError(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		change  func(*authenticationRepositoryFake)
		secret  string
		outcome PasswordAuthenticationOutcome
	}{
		{"wrong password", func(*authenticationRepositoryFake) {}, "different horse battery staple", PasswordAuthenticationWrongPassword},
		{"unknown email", func(r *authenticationRepositoryFake) { r.emailErr = ErrNotFound }, "correct horse battery staple", PasswordAuthenticationUnknownAccount},
		{"suspended account", func(r *authenticationRepositoryFake) { r.user.Status = UserSuspended }, "correct horse battery staple", PasswordAuthenticationSuspendedAccount},
		{"missing credential", func(r *authenticationRepositoryFake) { r.credentialErr = ErrNotFound }, "correct horse battery staple", PasswordAuthenticationMissingCredential},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repository, verifier := realAuthenticationFixture(t)
			scenario.change(repository)
			observer := &authenticationObserverFake{}
			service := NewPasswordAuthenticator(repository, verifier, observer, nil)
			result, err := service.AuthenticatePassword(context.Background(), "admin@example.com", []byte(scenario.secret))
			if err != ErrInvalidCredentials || result != (AuthenticatedIdentity{}) {
				t.Fatalf("ordinary failure returned distinct result: %+v, %v", result, err)
			}
			if strings.Contains(err.Error(), scenario.secret) {
				t.Fatal("authentication error exposed plaintext password")
			}
			if len(observer.attempts) != 1 || observer.attempts[0].Outcome != scenario.outcome {
				t.Fatalf("security outcome mismatch: %+v", observer.attempts)
			}
			if signal := fmt.Sprintf("%+v", observer.attempts[0]); strings.Contains(signal, scenario.secret) || strings.Contains(signal, "admin@example.com") {
				t.Fatal("security signal exposed submitted email or password")
			}
			if scenario.name == "unknown email" && observer.attempts[0].UserID != "" {
				t.Fatal("unknown account was assigned a synthetic user ID")
			}
		})
	}
}

func TestPasswordAuthenticatorUsesDummyVerifierForUnknownAndMissingAccounts(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*authenticationRepositoryFake)
	}{
		{"unknown", func(r *authenticationRepositoryFake) { r.emailErr = ErrNotFound }},
		{"missing credential", func(r *authenticationRepositoryFake) { r.credentialErr = ErrNotFound }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repository, _ := realAuthenticationFixture(t)
			scenario.change(repository)
			verifier := &passwordVerifierSpy{}
			service := NewPasswordAuthenticator(repository, verifier, nil, nil)
			_, err := service.AuthenticatePassword(context.Background(), "admin@example.com", []byte("short"))
			if err != ErrInvalidCredentials || len(verifier.calledWith) != 1 || verifier.calledWith[0].Value() != dummyPasswordPHC {
				t.Fatalf("dummy verification was skipped: calls=%d err=%v", len(verifier.calledWith), err)
			}
		})
	}
}

func TestPasswordAuthenticatorOperationalFailuresStayDistinct(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*authenticationRepositoryFake)
		want   error
	}{
		{"email query", func(r *authenticationRepositoryFake) { r.emailErr = errors.New("database detail") }, ErrAuthenticationUnavailable},
		{"user query", func(r *authenticationRepositoryFake) { r.userErr = errors.New("database detail") }, ErrAuthenticationUnavailable},
		{"credential query", func(r *authenticationRepositoryFake) { r.credentialErr = errors.New("database detail") }, ErrAuthenticationUnavailable},
		{"malformed stored hash", func(r *authenticationRepositoryFake) { r.credential.PasswordHash, _ = NewPasswordHash("malformed") }, ErrCorruptPasswordCredential},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repository, verifier := realAuthenticationFixture(t)
			scenario.change(repository)
			observer := &authenticationObserverFake{}
			service := NewPasswordAuthenticator(repository, verifier, observer, nil)
			result, err := service.AuthenticatePassword(context.Background(), "admin@example.com", []byte("correct horse battery staple"))
			if err != scenario.want || result != (AuthenticatedIdentity{}) || strings.Contains(err.Error(), "database detail") {
				t.Fatalf("operational failure was misclassified or leaked detail: %+v, %v", result, err)
			}
			if len(observer.attempts) != 1 || observer.attempts[0].UserID == "" && scenario.name != "email query" {
				t.Fatalf("operational signal missing: %+v", observer.attempts)
			}
			if scenario.name == "malformed stored hash" && observer.attempts[0].Outcome != PasswordAuthenticationCorruptCredential {
				t.Fatalf("corrupt credential signal missing: %+v", observer.attempts)
			}
		})
	}
}

func TestPasswordAuthenticatorVerifierFailureIsOperational(t *testing.T) {
	repository, _ := realAuthenticationFixture(t)
	observer := &authenticationObserverFake{}
	verifier := &passwordVerifierSpy{err: errors.New("private verifier detail")}
	service := NewPasswordAuthenticator(repository, verifier, observer, nil)
	_, err := service.AuthenticatePassword(context.Background(), "admin@example.com", []byte("correct horse battery staple"))
	if err != ErrAuthenticationUnavailable || strings.Contains(err.Error(), "private verifier detail") || len(observer.attempts) != 1 || observer.attempts[0].Outcome != PasswordAuthenticationVerifierFailure {
		t.Fatalf("verifier failure was not safely classified: %v, %+v", err, observer.attempts)
	}
}

func TestDummyHashIsAValidPrecomputedArgon2idHash(t *testing.T) {
	valid, err := DefaultPasswordHasher().VerifyPassword(dummyPasswordHash, []byte("arbitrary submitted passphrase"))
	if err != nil || valid {
		t.Fatalf("dummy hash was not a valid work factor: verified=%v err=%v", valid, err)
	}
	valid, err = DefaultPasswordHasher().VerifyPassword(dummyPasswordHash, []byte("short"))
	if err != nil || valid {
		t.Fatalf("short input skipped dummy Argon2id verification: verified=%v err=%v", valid, err)
	}
}
