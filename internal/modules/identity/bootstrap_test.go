package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type bootstrapRepositoryFake struct {
	userCalls, emailCalls, verifyCalls, credentialCalls, roleCalls int
	emailErr, roleErr                                              error
}

func (f *bootstrapRepositoryFake) CreateUser(context.Context, UserStatus) (User, error) {
	f.userCalls++
	return User{ID: "00000000-0000-0000-0000-000000000001", Status: UserActive}, nil
}
func (f *bootstrapRepositoryFake) GetUser(context.Context, UserID) (User, error) {
	return User{}, ErrNotFound
}
func (f *bootstrapRepositoryFake) UpdateUserStatus(context.Context, UserID, UserStatus) (User, error) {
	return User{}, nil
}
func (f *bootstrapRepositoryFake) CreateUserEmail(_ context.Context, userID UserID, email string, primary bool) (UserEmail, error) {
	f.emailCalls++
	if f.emailErr != nil {
		return UserEmail{}, f.emailErr
	}
	return UserEmail{ID: "00000000-0000-0000-0000-000000000002", UserID: userID, DisplayEmail: email, IsPrimary: primary}, nil
}
func (f *bootstrapRepositoryFake) GetUserEmailByNormalized(context.Context, string) (UserEmail, error) {
	return UserEmail{}, ErrNotFound
}
func (f *bootstrapRepositoryFake) GetPrimaryUserEmail(context.Context, UserID) (UserEmail, error) {
	return UserEmail{}, ErrNotFound
}
func (f *bootstrapRepositoryFake) MarkUserEmailVerified(_ context.Context, id EmailID, at time.Time) (UserEmail, error) {
	f.verifyCalls++
	return UserEmail{ID: id, VerifiedAt: &at}, nil
}
func (f *bootstrapRepositoryFake) CreateLocalPasswordCredential(_ context.Context, userID UserID, hash PasswordHash) (LocalPasswordCredential, error) {
	f.credentialCalls++
	return LocalPasswordCredential{UserID: userID, PasswordHash: hash}, nil
}
func (f *bootstrapRepositoryFake) GetLocalPasswordCredential(context.Context, UserID) (LocalPasswordCredential, error) {
	return LocalPasswordCredential{}, ErrNotFound
}
func (f *bootstrapRepositoryFake) ReplaceLocalPasswordHash(context.Context, UserID, PasswordHash) (LocalPasswordCredential, error) {
	return LocalPasswordCredential{}, nil
}
func (f *bootstrapRepositoryFake) AssignGlobalRole(_ context.Context, userID UserID, role GlobalRole, _ *UserID) (GlobalRoleAssignment, error) {
	f.roleCalls++
	if f.roleErr != nil {
		return GlobalRoleAssignment{}, f.roleErr
	}
	return GlobalRoleAssignment{UserID: userID, Role: role}, nil
}
func (f *bootstrapRepositoryFake) ListActiveGlobalRoles(context.Context, UserID) ([]GlobalRoleAssignment, error) {
	return nil, nil
}
func (f *bootstrapRepositoryFake) HasActiveGlobalRole(context.Context, UserID, GlobalRole) (bool, error) {
	return false, nil
}
func (f *bootstrapRepositoryFake) RevokeGlobalRole(context.Context, UserID, GlobalRole) (GlobalRoleAssignment, error) {
	return GlobalRoleAssignment{}, nil
}

type bootstrapAuditFake struct {
	calls int
	err   error
}

func (f *bootstrapAuditFake) RecordAdministratorBootstrap(context.Context, string, string) error {
	f.calls++
	return f.err
}

type failingHasher struct{ err error }

func (h failingHasher) HashPassword([]byte) (PasswordHash, error)         { return PasswordHash{}, h.err }
func (h failingHasher) VerifyPassword(PasswordHash, []byte) (bool, error) { return false, h.err }

func TestAdministratorBootstrapperCreatesRequiredRecords(t *testing.T) {
	repositories := &bootstrapRepositoryFake{}
	audit := &bootstrapAuditFake{}
	service := NewAdministratorBootstrapper(testPasswordHasher(t), func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	password := []byte("correct horse battery staple")
	user, err := service.Bootstrap(context.Background(), repositories, audit, AdministratorBootstrapInput{Email: "Admin@example.com", Password: password, OperationID: "00000000-0000-0000-0000-000000000003"})
	if err != nil || user.Status != UserActive {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if repositories.userCalls != 1 || repositories.emailCalls != 1 || repositories.verifyCalls != 1 || repositories.credentialCalls != 1 || repositories.roleCalls != 1 || audit.calls != 1 {
		t.Fatalf("required bootstrap calls missing: %+v audit=%d", repositories, audit.calls)
	}
	if strings.Trim(string(password), "\x00") != "" {
		t.Fatal("plaintext password was not cleared after use")
	}
}

func TestAdministratorBootstrapperStopsOnConflictWithoutCompletion(t *testing.T) {
	repositories := &bootstrapRepositoryFake{emailErr: ErrConflict}
	audit := &bootstrapAuditFake{}
	service := NewAdministratorBootstrapper(testPasswordHasher(t), nil)
	_, err := service.Bootstrap(context.Background(), repositories, audit, AdministratorBootstrapInput{Email: "Admin@example.com", Password: []byte("correct horse battery staple"), OperationID: "operation"})
	if !errors.Is(err, ErrAdministratorAlreadyExists) {
		t.Fatalf("expected conflict result, got %v", err)
	}
	if repositories.credentialCalls != 0 || repositories.roleCalls != 0 || audit.calls != 0 {
		t.Fatal("bootstrap continued after duplicate email")
	}
}

func TestAdministratorBootstrapperHashesBeforePersistenceAndRedactsErrors(t *testing.T) {
	repositories := &bootstrapRepositoryFake{}
	service := NewAdministratorBootstrapper(failingHasher{err: errors.New("secret password was unavailable")}, nil)
	_, err := service.Bootstrap(context.Background(), repositories, &bootstrapAuditFake{}, AdministratorBootstrapInput{Email: "Admin@example.com", Password: []byte("correct horse battery staple"), OperationID: "operation"})
	if !errors.Is(err, ErrAdministratorBootstrapFailed) || strings.Contains(err.Error(), "secret password") {
		t.Fatalf("unsafe hashing error: %v", err)
	}
	if repositories.userCalls != 0 {
		t.Fatal("persistence started after hashing failure")
	}
}

func TestAdministratorBootstrapperStopsOnRoleFailure(t *testing.T) {
	repositories := &bootstrapRepositoryFake{roleErr: errors.New("role failure")}
	audit := &bootstrapAuditFake{}
	service := NewAdministratorBootstrapper(testPasswordHasher(t), nil)
	_, err := service.Bootstrap(context.Background(), repositories, audit, AdministratorBootstrapInput{Email: "Admin@example.com", Password: []byte("correct horse battery staple"), OperationID: "operation"})
	if !errors.Is(err, ErrAdministratorBootstrapFailed) || audit.calls != 0 {
		t.Fatalf("role failure did not stop bootstrap safely: %v", err)
	}
}
