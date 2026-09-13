package identity

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrAdministratorAlreadyExists = errors.New("administrator account already exists")
var ErrAdministratorBootstrapFailed = errors.New("administrator bootstrap failed")

type AdministratorBootstrapRepositories interface {
	UserRepository
	EmailRepository
	PasswordCredentialRepository
	GlobalRoleRepository
}

// AdministratorBootstrapAuditRecorder keeps the Identity service independent
// of the Audit module while requiring the audit fact in the operation contract.
type AdministratorBootstrapAuditRecorder interface {
	RecordAdministratorBootstrap(context.Context, string, string) error
}

type AdministratorBootstrapInput struct {
	Email       string
	Password    []byte
	OperationID string
}

func (AdministratorBootstrapInput) String() string { return "AdministratorBootstrapInput{[REDACTED]}" }
func (AdministratorBootstrapInput) GoString() string {
	return "AdministratorBootstrapInput{[REDACTED]}"
}
func (AdministratorBootstrapInput) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("AdministratorBootstrapInput{[REDACTED]}"))
}

type AdministratorBootstrapper struct {
	hasher PasswordHasher
	now    func() time.Time
}

func NewAdministratorBootstrapper(hasher PasswordHasher, now func() time.Time) AdministratorBootstrapper {
	if now == nil {
		now = time.Now
	}
	return AdministratorBootstrapper{hasher: hasher, now: now}
}

func (b AdministratorBootstrapper) Bootstrap(ctx context.Context, repositories AdministratorBootstrapRepositories, audit AdministratorBootstrapAuditRecorder, input AdministratorBootstrapInput) (User, error) {
	defer clear(input.Password)
	if b.hasher == nil || repositories == nil || audit == nil {
		return User{}, ErrAdministratorBootstrapFailed
	}
	if _, err := NormalizeEmail(input.Email); err != nil {
		return User{}, err
	}
	if err := ValidatePassword(input.Password); err != nil {
		return User{}, err
	}
	if input.OperationID == "" {
		return User{}, ErrAdministratorBootstrapFailed
	}
	hash, err := b.hasher.HashPassword(input.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			return User{}, err
		}
		return User{}, ErrAdministratorBootstrapFailed
	}
	user, err := repositories.CreateUser(ctx, UserActive)
	if err != nil {
		return User{}, bootstrapError(err)
	}
	email, err := repositories.CreateUserEmail(ctx, user.ID, input.Email, true)
	if err != nil {
		return User{}, bootstrapError(err)
	}
	if _, err := repositories.MarkUserEmailVerified(ctx, email.ID, b.now().UTC()); err != nil {
		return User{}, bootstrapError(err)
	}
	if _, err := repositories.CreateLocalPasswordCredential(ctx, user.ID, hash); err != nil {
		return User{}, bootstrapError(err)
	}
	if _, err := repositories.AssignGlobalRole(ctx, user.ID, RoleAdministrator, nil); err != nil {
		return User{}, bootstrapError(err)
	}
	if err := audit.RecordAdministratorBootstrap(ctx, string(user.ID), input.OperationID); err != nil {
		return User{}, ErrAdministratorBootstrapFailed
	}
	return user, nil
}

func bootstrapError(err error) error {
	if errors.Is(err, ErrConflict) {
		return ErrAdministratorAlreadyExists
	}
	return ErrAdministratorBootstrapFailed
}
