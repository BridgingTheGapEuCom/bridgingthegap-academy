package identity

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

type UserID string
type EmailID string
type RoleAssignmentID string
type SessionID string

type UserStatus string

const (
	UserActive    UserStatus = "ACTIVE"
	UserSuspended UserStatus = "SUSPENDED"
)

func ParseUserStatus(value string) (UserStatus, error) {
	status := UserStatus(value)
	if status != UserActive && status != UserSuspended {
		return "", fmt.Errorf("invalid user status %q", value)
	}
	return status, nil
}

type GlobalRole string

const RoleAdministrator GlobalRole = "ADMINISTRATOR"

func ParseGlobalRole(value string) (GlobalRole, error) {
	role := GlobalRole(value)
	if role != RoleAdministrator {
		return "", fmt.Errorf("invalid global role %q", value)
	}
	return role, nil
}

// NormalizeEmail defines the lookup key; display spelling is retained separately.
func NormalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	local, domain, hasAt := strings.Cut(normalized, "@")
	if !hasAt || local == "" || domain == "" || strings.Contains(domain, "@") || strings.IndexFunc(normalized, unicode.IsSpace) >= 0 {
		return "", errors.New("invalid email")
	}
	return normalized, nil
}

type User struct {
	ID        UserID
	Status    UserStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserEmail struct {
	ID              EmailID
	UserID          UserID
	NormalizedEmail string
	DisplayEmail    string
	VerifiedAt      *time.Time
	IsPrimary       bool
	CreatedAt       time.Time
}

// PasswordHash is opaque to unrelated domain APIs and redacts diagnostic formatting.
type PasswordHash struct{ value string }

func NewPasswordHash(value string) (PasswordHash, error) {
	if value == "" {
		return PasswordHash{}, errors.New("empty password hash")
	}
	return PasswordHash{value: value}, nil
}

func (h PasswordHash) Value() string                 { return h.value }
func (h PasswordHash) String() string                { return "[REDACTED]" }
func (h PasswordHash) GoString() string              { return "[REDACTED]" }
func (h PasswordHash) Format(s fmt.State, verb rune) { _, _ = s.Write([]byte("[REDACTED]")) }

type LocalPasswordCredential struct {
	UserID       UserID
	PasswordHash PasswordHash
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type GlobalRoleAssignment struct {
	ID        RoleAssignmentID
	UserID    UserID
	Role      GlobalRole
	GrantedAt time.Time
	GrantedBy *UserID
	RevokedAt *time.Time
}

// SessionTokenDigest is a SHA-256 digest of an opaque token, never the token itself.
type SessionTokenDigest struct{ value [32]byte }

func NewSessionTokenDigest(value []byte) (SessionTokenDigest, error) {
	if len(value) != 32 {
		return SessionTokenDigest{}, errors.New("session token digest must be 32 bytes")
	}
	var digest SessionTokenDigest
	copy(digest.value[:], value)
	return digest, nil
}

func (d SessionTokenDigest) Bytes() []byte                 { return append([]byte(nil), d.value[:]...) }
func (d SessionTokenDigest) String() string                { return "[REDACTED]" }
func (d SessionTokenDigest) GoString() string              { return "[REDACTED]" }
func (d SessionTokenDigest) Format(s fmt.State, verb rune) { _, _ = s.Write([]byte("[REDACTED]")) }

type Session struct {
	ID          SessionID
	UserID      UserID
	TokenDigest SessionTokenDigest
	CreatedAt   time.Time
	LastSeenAt  time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}
