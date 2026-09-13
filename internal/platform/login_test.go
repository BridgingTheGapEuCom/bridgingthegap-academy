package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

const loginTestUser identity.UserID = "00000000-0000-0000-0000-000000000001"
const loginTestSession identity.SessionID = "00000000-0000-0000-0000-000000000002"
const loginTestOtherSession identity.SessionID = "00000000-0000-0000-0000-000000000003"
const loginTestOperation = "00000000-0000-0000-0000-000000000004"

type loginAuthenticatorFake struct {
	actor identity.AuthenticatedIdentity
	err   error
	calls int
}

func (f *loginAuthenticatorFake) AuthenticatePassword(_ context.Context, _ string, _ []byte) (identity.AuthenticatedIdentity, error) {
	f.calls++
	return f.actor, f.err
}

type completedTransactionFake struct {
	created        identity.CreatedSession
	createErr      error
	revokeErr      error
	auditErr       error
	commitErr      error
	beginCount     int
	createCount    int
	committed      []audit.Event
	revoked        map[identity.SessionID]bool
	lastAttempt    audit.Event
	lastRevoke     identity.SessionID
	lastCreateUser identity.UserID
}

type completedWorkFake struct {
	runner  *completedTransactionFake
	events  []audit.Event
	revoked []identity.SessionID
}

func (f *completedTransactionFake) InCompletedSessionTransaction(ctx context.Context, fn func(CompletedSessionTransaction) error) error {
	f.beginCount++
	work := &completedWorkFake{runner: f}
	if err := fn(work); err != nil {
		return err // Simulated rollback: staged changes are discarded.
	}
	if f.commitErr != nil {
		return f.commitErr
	}
	f.committed = append(f.committed, work.events...)
	if f.revoked == nil {
		f.revoked = make(map[identity.SessionID]bool)
	}
	for _, id := range work.revoked {
		f.revoked[id] = true
	}
	return nil
}
func (f *completedWorkFake) CreateSession(_ context.Context, userID identity.UserID) (identity.CreatedSession, error) {
	f.runner.createCount++
	f.runner.lastCreateUser = userID
	return f.runner.created, f.runner.createErr
}
func (f *completedWorkFake) RevokeResolvedSession(_ context.Context, current identity.ResolvedSession) (bool, error) {
	sessionID := current.SessionID()
	f.runner.lastRevoke = sessionID
	if f.runner.revokeErr != nil {
		return false, f.runner.revokeErr
	}
	if f.runner.revoked[sessionID] {
		return false, nil
	}
	f.revoked = append(f.revoked, sessionID)
	return true, nil
}
func (f *completedWorkFake) AppendAudit(_ context.Context, event audit.Event) error {
	f.runner.lastAttempt = event
	if f.runner.auditErr != nil {
		return f.runner.auditErr
	}
	f.events = append(f.events, event)
	return nil
}
func (f *completedWorkFake) GetAuditByOperationID(_ context.Context, operationID string) (audit.Event, error) {
	for _, event := range f.runner.committed {
		if event.OperationID == operationID {
			return event, nil
		}
	}
	return audit.Event{}, audit.ErrNotFound
}

func newLoginFixture() (*loginAuthenticatorFake, *completedTransactionFake, LoginOrchestrator) {
	at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	authenticator := &loginAuthenticatorFake{actor: identity.AuthenticatedIdentity{UserID: loginTestUser, Method: identity.AuthenticationMethodLocalPassword, AuthenticatedAt: at}}
	transactions := &completedTransactionFake{created: identity.CreatedSession{Token: mustRawToken(), SessionID: loginTestSession, UserID: loginTestUser, IssuedAt: at, ExpiresAt: at.Add(time.Hour)}}
	return authenticator, transactions, NewLoginOrchestrator(authenticator, transactions)
}

func mustRawToken() identity.RawSessionToken {
	// The token constructor belongs to SessionService; create a token through a
	// focused fake repository instead of adding a public token constructor.
	repository := &loginSessionRepositoryFake{at: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)}
	created, err := identity.NewSessionService(repository, nil, func() time.Time { return repository.at }).CreateSession(context.Background(), loginTestUser)
	if err != nil {
		panic(err)
	}
	return created.Token
}

type loginSessionRepositoryFake struct {
	at      time.Time
	session identity.Session
}

func (r *loginSessionRepositoryFake) GetUser(context.Context, identity.UserID) (identity.User, error) {
	return identity.User{ID: loginTestUser, Status: identity.UserActive}, nil
}
func (r *loginSessionRepositoryFake) CreateSession(_ context.Context, userID identity.UserID, digest identity.SessionTokenDigest, expires time.Time) (identity.Session, error) {
	r.session = identity.Session{ID: loginTestSession, UserID: userID, TokenDigest: digest, CreatedAt: r.at, LastSeenAt: r.at, ExpiresAt: expires}
	return r.session, nil
}
func (r *loginSessionRepositoryFake) GetSessionByDigest(_ context.Context, digest identity.SessionTokenDigest) (identity.Session, error) {
	if bytes.Equal(r.session.TokenDigest.Bytes(), digest.Bytes()) {
		return r.session, nil
	}
	return identity.Session{}, identity.ErrNotFound
}
func (r *loginSessionRepositoryFake) GetSessionByID(_ context.Context, id identity.SessionID) (identity.Session, error) {
	if r.session.ID == id {
		return r.session, nil
	}
	return identity.Session{}, identity.ErrNotFound
}
func (r *loginSessionRepositoryFake) RevokeSession(context.Context, identity.SessionID) (identity.Session, error) {
	return identity.Session{}, identity.ErrNotFound
}
func (r *loginSessionRepositoryFake) RevokeUserSessions(context.Context, identity.UserID) (int64, error) {
	return 0, nil
}

func loginTestCurrent(t *testing.T) identity.ResolvedSession {
	t.Helper()
	repository := &loginSessionRepositoryFake{at: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)}
	service := identity.NewSessionService(repository, nil, func() time.Time { return repository.at })
	created, err := service.CreateSession(context.Background(), loginTestUser)
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.ResolveSession(context.Background(), created.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	return current
}

func TestLoginOrchestratorSuccessIsCompletedOnlyAfterAuditCommit(t *testing.T) {
	authenticator, transactions, service := newLoginFixture()
	password := []byte("correct horse battery staple")
	result, err := service.LoginWithPassword(context.Background(), "admin@example.com", password, loginTestOperation)
	if err != nil || result.UserID != loginTestUser || result.SessionID != loginTestSession || result.Token.Value() == "" || result.OperationID != loginTestOperation || result.Method != identity.AuthenticationMethodLocalPassword {
		t.Fatalf("login result is incomplete: %v", err)
	}
	if authenticator.calls != 1 || transactions.beginCount != 1 || transactions.createCount != 1 || transactions.lastCreateUser != loginTestUser || len(transactions.committed) != 1 {
		t.Fatal("login did not authenticate, create one session, and commit one audit fact")
	}
	event := transactions.committed[0]
	if event.Action != audit.LocalPasswordLoginCompleted || event.ActorKind != "USER" || event.ActorUserID != string(loginTestUser) || event.ResourceType != "IDENTITY_SESSION" || event.ResourceID != string(loginTestSession) || event.AuthenticationMethod != string(identity.AuthenticationMethodLocalPassword) || event.OperationID != loginTestOperation || event.Outcome != "SUCCESS" {
		t.Fatalf("completed login audit fact is wrong: %+v", event)
	}
	if strings.Contains(fmt.Sprintf("%+v", event), result.Token.Value()) || strings.Contains(fmt.Sprintf("%+v", event), "correct horse battery staple") || strings.Contains(fmt.Sprintf("%+v", result), result.Token.Value()) {
		t.Fatal("login audit/result formatting leaked a bearer secret")
	}
	if strings.Trim(string(password), "\x00") != "" {
		t.Fatal("login did not clear its password input")
	}
}

func TestLoginOrchestratorAuthenticationFailuresDoNotCreateSession(t *testing.T) {
	for _, scenario := range []struct {
		name string
		err  error
	}{
		{"wrong password", identity.ErrInvalidCredentials},
		{"unknown account", identity.ErrInvalidCredentials},
		{"suspended account", identity.ErrInvalidCredentials},
		{"authentication storage failure", identity.ErrAuthenticationUnavailable},
		{"corrupt credential", identity.ErrCorruptPasswordCredential},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			authenticator, transactions, service := newLoginFixture()
			authenticator.err = scenario.err
			result, err := service.LoginWithPassword(context.Background(), "admin@example.com", []byte("private password"), loginTestOperation)
			if err != scenario.err || result != (LoginResult{}) || transactions.beginCount != 0 || transactions.createCount != 0 || len(transactions.committed) != 0 {
				t.Fatalf("authentication failure created a login: %v", err)
			}
		})
	}
}

func TestLoginOrchestratorRollsBackOnSessionAuditOrCommitFailure(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		setup func(*completedTransactionFake)
	}{
		{"session insert", func(f *completedTransactionFake) { f.createErr = errors.New("private session storage detail") }},
		{"audit append", func(f *completedTransactionFake) { f.auditErr = errors.New("private audit storage detail") }},
		{"transaction commit", func(f *completedTransactionFake) { f.commitErr = errors.New("private commit detail") }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			_, transactions, service := newLoginFixture()
			scenario.setup(transactions)
			result, err := service.LoginWithPassword(context.Background(), "admin@example.com", []byte("private password"), loginTestOperation)
			if err != ErrLoginUnavailable || result != (LoginResult{}) || len(transactions.committed) != 0 || strings.Contains(err.Error(), "private") {
				t.Fatalf("failed transaction reported successful login: %v", err)
			}
			if scenario.name != "session insert" && transactions.createCount != 1 {
				t.Fatal("failure did not occur after session creation")
			}
		})
	}
}

func TestLogoutOrchestratorRevokesOnlyCurrentSessionAndAuditsOneCompletion(t *testing.T) {
	_, transactions, service := newLoginFixture()
	current := loginTestCurrent(t)
	for field := range reflect.TypeOf(current).NumField() {
		if reflect.TypeOf(current).Field(field).IsExported() {
			t.Fatal("trusted session context has externally writable fields")
		}
	}
	transactions.revoked = map[identity.SessionID]bool{loginTestOtherSession: false}
	if err := service.Logout(context.Background(), current, loginTestOperation); err != nil {
		t.Fatal(err)
	}
	if !transactions.revoked[loginTestSession] || transactions.revoked[loginTestOtherSession] || len(transactions.committed) != 1 {
		t.Fatal("logout did not revoke only the current session")
	}
	event := transactions.committed[0]
	if event.Action != audit.SessionLogoutCompleted || event.ActorUserID != string(loginTestUser) || event.ResourceID != string(loginTestSession) || event.OperationID != loginTestOperation || event.AuthenticationMethod != "" {
		t.Fatalf("logout audit fact is wrong: %+v", event)
	}
	if err := service.Logout(context.Background(), current, "00000000-0000-0000-0000-000000000005"); err != nil || len(transactions.committed) != 1 {
		t.Fatalf("repeated logout was not safe: %v", err)
	}
	if err := service.Logout(context.Background(), current, loginTestOperation); err != nil || len(transactions.committed) != 1 {
		t.Fatalf("retry with same operation ID was not idempotent: %v", err)
	}
	if transactions.committed[0].OperationID != loginTestOperation {
		t.Fatal("operation ID was not propagated")
	}
	if err := service.Logout(context.Background(), identity.ResolvedSession{}, ""); err != identity.ErrInvalidSession {
		t.Fatalf("untrusted empty session accepted: %v", err)
	}
}

func TestLogoutOrchestratorRollsBackWhenRevocationOrAuditFails(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		setup func(*completedTransactionFake)
	}{
		{"revocation", func(f *completedTransactionFake) { f.revokeErr = errors.New("private revoke detail") }},
		{"audit", func(f *completedTransactionFake) { f.auditErr = errors.New("private audit detail") }},
		{"commit", func(f *completedTransactionFake) { f.commitErr = errors.New("private commit detail") }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			_, transactions, service := newLoginFixture()
			transactions.revoked = make(map[identity.SessionID]bool)
			scenario.setup(transactions)
			err := service.Logout(context.Background(), loginTestCurrent(t), loginTestOperation)
			if err != ErrLogoutUnavailable || transactions.revoked[loginTestSession] || len(transactions.committed) != 0 || strings.Contains(err.Error(), "private") {
				t.Fatalf("failed logout was not rolled back: %v", err)
			}
		})
	}
}

func TestLogoutRejectsOperationIDOwnedByAnotherSession(t *testing.T) {
	for _, alreadyRevoked := range []bool{false, true} {
		_, transactions, service := newLoginFixture()
		transactions.revoked = map[identity.SessionID]bool{loginTestSession: alreadyRevoked}
		transactions.committed = []audit.Event{{Action: audit.SessionLogoutCompleted, ActorUserID: string(loginTestUser), ResourceType: "IDENTITY_SESSION", ResourceID: string(loginTestOtherSession), Outcome: "SUCCESS", OperationID: loginTestOperation}}
		err := service.Logout(context.Background(), loginTestCurrent(t), loginTestOperation)
		if err != ErrLogoutUnavailable || transactions.revoked[loginTestSession] != alreadyRevoked || len(transactions.committed) != 1 {
			t.Fatalf("operation ID collision changed another logout (already revoked=%t): %v", alreadyRevoked, err)
		}
	}
}
