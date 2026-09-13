package identity

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("identity record not found")
var ErrConflict = errors.New("identity record conflicts with existing data")

type UserRepository interface {
	CreateUser(context.Context, UserStatus) (User, error)
	GetUser(context.Context, UserID) (User, error)
	UpdateUserStatus(context.Context, UserID, UserStatus) (User, error)
}

type EmailRepository interface {
	CreateUserEmail(context.Context, UserID, string, bool) (UserEmail, error)
	GetUserEmailByNormalized(context.Context, string) (UserEmail, error)
	GetPrimaryUserEmail(context.Context, UserID) (UserEmail, error)
	MarkUserEmailVerified(context.Context, EmailID, time.Time) (UserEmail, error)
}

type PasswordCredentialRepository interface {
	CreateLocalPasswordCredential(context.Context, UserID, PasswordHash) (LocalPasswordCredential, error)
	GetLocalPasswordCredential(context.Context, UserID) (LocalPasswordCredential, error)
	ReplaceLocalPasswordHash(context.Context, UserID, PasswordHash) (LocalPasswordCredential, error)
}

type GlobalRoleRepository interface {
	AssignGlobalRole(context.Context, UserID, GlobalRole, *UserID) (GlobalRoleAssignment, error)
	ListActiveGlobalRoles(context.Context, UserID) ([]GlobalRoleAssignment, error)
	HasActiveGlobalRole(context.Context, UserID, GlobalRole) (bool, error)
	RevokeGlobalRole(context.Context, UserID, GlobalRole) (GlobalRoleAssignment, error)
}

type SessionRepository interface {
	CreateSession(context.Context, UserID, SessionTokenDigest, time.Time) (Session, error)
	GetSessionByDigest(context.Context, SessionTokenDigest) (Session, error)
	RevokeSession(context.Context, SessionID) (Session, error)
	RevokeUserSessions(context.Context, UserID) (int64, error)
	UpdateSessionLastSeen(context.Context, SessionID) (Session, error)
}
