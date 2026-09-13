package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository implements Identity-owned contracts using only Identity tables.
type Repository struct{ q *sqlc.Queries }

var _ identity.UserRepository = (*Repository)(nil)
var _ identity.EmailRepository = (*Repository)(nil)
var _ identity.PasswordCredentialRepository = (*Repository)(nil)
var _ identity.GlobalRoleRepository = (*Repository)(nil)
var _ identity.SessionRepository = (*Repository)(nil)
var _ identity.SessionLifecycleRepository = (*Repository)(nil)

func New(db sqlc.DBTX) *Repository { return &Repository{q: sqlc.New(db)} }

func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, errors.New("invalid identity identifier")
	}
	return id, nil
}

func nullableUUID(value *identity.UserID) (pgtype.UUID, error) {
	if value == nil {
		return pgtype.UUID{}, nil
	}
	return uuid(string(*value))
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func storageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.ErrNotFound
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return identity.ErrConflict
		case "23503":
			return identity.ErrNotFound
		case "23514", "23502":
			return errors.New("identity constraint violation")
		}
		return fmt.Errorf("identity storage failure (SQLSTATE %s)", pgErr.Code)
	}
	return errors.New("identity storage failure")
}

func mapUser(row sqlc.IdentityUser) (identity.User, error) {
	status, err := identity.ParseUserStatus(row.Status)
	return identity.User{ID: identity.UserID(row.ID.String()), Status: status, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, err
}

func mapEmail(row sqlc.IdentityUserEmail) identity.UserEmail {
	return identity.UserEmail{ID: identity.EmailID(row.ID.String()), UserID: identity.UserID(row.UserID.String()), NormalizedEmail: row.NormalizedEmail, DisplayEmail: row.DisplayEmail, VerifiedAt: optionalTime(row.VerifiedAt), IsPrimary: row.IsPrimary, CreatedAt: row.CreatedAt.Time}
}

func mapCredential(row sqlc.IdentityLocalPasswordCredential) (identity.LocalPasswordCredential, error) {
	hash, err := identity.NewPasswordHash(row.PasswordHash)
	return identity.LocalPasswordCredential{UserID: identity.UserID(row.UserID.String()), PasswordHash: hash, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, err
}

func mapRole(row sqlc.IdentityGlobalRoleAssignment) (identity.GlobalRoleAssignment, error) {
	role, err := identity.ParseGlobalRole(row.Role)
	var grantedBy *identity.UserID
	if row.GrantedBy.Valid {
		id := identity.UserID(row.GrantedBy.String())
		grantedBy = &id
	}
	return identity.GlobalRoleAssignment{ID: identity.RoleAssignmentID(row.ID.String()), UserID: identity.UserID(row.UserID.String()), Role: role, GrantedAt: row.GrantedAt.Time, GrantedBy: grantedBy, RevokedAt: optionalTime(row.RevokedAt)}, err
}

func mapSession(row sqlc.IdentitySession) (identity.Session, error) {
	digest, err := identity.NewSessionTokenDigest(row.TokenDigest)
	return identity.Session{ID: identity.SessionID(row.ID.String()), UserID: identity.UserID(row.UserID.String()), TokenDigest: digest, CreatedAt: row.CreatedAt.Time, LastSeenAt: row.LastSeenAt.Time, ExpiresAt: row.ExpiresAt.Time, RevokedAt: optionalTime(row.RevokedAt)}, err
}

func (r *Repository) CreateUser(ctx context.Context, status identity.UserStatus) (identity.User, error) {
	if _, err := identity.ParseUserStatus(string(status)); err != nil {
		return identity.User{}, err
	}
	row, err := r.q.CreateUser(ctx, string(status))
	if err != nil {
		return identity.User{}, storageError(err)
	}
	return mapUser(row)
}
func (r *Repository) GetUser(ctx context.Context, id identity.UserID) (identity.User, error) {
	key, err := uuid(string(id))
	if err != nil {
		return identity.User{}, err
	}
	row, err := r.q.GetUser(ctx, key)
	if err != nil {
		return identity.User{}, storageError(err)
	}
	return mapUser(row)
}
func (r *Repository) UpdateUserStatus(ctx context.Context, id identity.UserID, status identity.UserStatus) (identity.User, error) {
	if _, err := identity.ParseUserStatus(string(status)); err != nil {
		return identity.User{}, err
	}
	key, err := uuid(string(id))
	if err != nil {
		return identity.User{}, err
	}
	row, err := r.q.UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{ID: key, Status: string(status)})
	if err != nil {
		return identity.User{}, storageError(err)
	}
	return mapUser(row)
}
func (r *Repository) CreateUserEmail(ctx context.Context, userID identity.UserID, email string, primary bool) (identity.UserEmail, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return identity.UserEmail{}, err
	}
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return identity.UserEmail{}, err
	}
	row, err := r.q.CreateUserEmail(ctx, sqlc.CreateUserEmailParams{UserID: key, NormalizedEmail: normalized, DisplayEmail: strings.TrimSpace(email), IsPrimary: primary})
	if err != nil {
		return identity.UserEmail{}, storageError(err)
	}
	return mapEmail(row), nil
}
func (r *Repository) GetUserEmailByNormalized(ctx context.Context, email string) (identity.UserEmail, error) {
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return identity.UserEmail{}, err
	}
	row, err := r.q.GetUserEmailByNormalized(ctx, normalized)
	if err != nil {
		return identity.UserEmail{}, storageError(err)
	}
	return mapEmail(row), nil
}
func (r *Repository) GetPrimaryUserEmail(ctx context.Context, userID identity.UserID) (identity.UserEmail, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return identity.UserEmail{}, err
	}
	row, err := r.q.GetPrimaryUserEmail(ctx, key)
	if err != nil {
		return identity.UserEmail{}, storageError(err)
	}
	return mapEmail(row), nil
}
func (r *Repository) MarkUserEmailVerified(ctx context.Context, id identity.EmailID, at time.Time) (identity.UserEmail, error) {
	key, err := uuid(string(id))
	if err != nil {
		return identity.UserEmail{}, err
	}
	row, err := r.q.MarkUserEmailVerified(ctx, sqlc.MarkUserEmailVerifiedParams{ID: key, VerifiedAt: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		return identity.UserEmail{}, storageError(err)
	}
	return mapEmail(row), nil
}
func (r *Repository) CreateLocalPasswordCredential(ctx context.Context, userID identity.UserID, hash identity.PasswordHash) (identity.LocalPasswordCredential, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return identity.LocalPasswordCredential{}, err
	}
	if hash.Value() == "" {
		return identity.LocalPasswordCredential{}, errors.New("empty password hash")
	}
	row, err := r.q.CreateLocalPasswordCredential(ctx, sqlc.CreateLocalPasswordCredentialParams{UserID: key, PasswordHash: hash.Value()})
	if err != nil {
		return identity.LocalPasswordCredential{}, storageError(err)
	}
	return mapCredential(row)
}
func (r *Repository) GetLocalPasswordCredential(ctx context.Context, userID identity.UserID) (identity.LocalPasswordCredential, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return identity.LocalPasswordCredential{}, err
	}
	row, err := r.q.GetLocalPasswordCredential(ctx, key)
	if err != nil {
		return identity.LocalPasswordCredential{}, storageError(err)
	}
	return mapCredential(row)
}
func (r *Repository) ReplaceLocalPasswordHash(ctx context.Context, userID identity.UserID, hash identity.PasswordHash) (identity.LocalPasswordCredential, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return identity.LocalPasswordCredential{}, err
	}
	if hash.Value() == "" {
		return identity.LocalPasswordCredential{}, errors.New("empty password hash")
	}
	row, err := r.q.ReplaceLocalPasswordHash(ctx, sqlc.ReplaceLocalPasswordHashParams{UserID: key, PasswordHash: hash.Value()})
	if err != nil {
		return identity.LocalPasswordCredential{}, storageError(err)
	}
	return mapCredential(row)
}
func (r *Repository) AssignGlobalRole(ctx context.Context, userID identity.UserID, role identity.GlobalRole, by *identity.UserID) (identity.GlobalRoleAssignment, error) {
	if _, err := identity.ParseGlobalRole(string(role)); err != nil {
		return identity.GlobalRoleAssignment{}, err
	}
	key, err := uuid(string(userID))
	if err != nil {
		return identity.GlobalRoleAssignment{}, err
	}
	granter, err := nullableUUID(by)
	if err != nil {
		return identity.GlobalRoleAssignment{}, err
	}
	row, err := r.q.AssignGlobalRole(ctx, sqlc.AssignGlobalRoleParams{UserID: key, Role: string(role), GrantedBy: granter})
	if err != nil {
		return identity.GlobalRoleAssignment{}, storageError(err)
	}
	return mapRole(row)
}
func (r *Repository) ListActiveGlobalRoles(ctx context.Context, userID identity.UserID) ([]identity.GlobalRoleAssignment, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListActiveGlobalRoles(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	roles := make([]identity.GlobalRoleAssignment, 0, len(rows))
	for _, row := range rows {
		role, err := mapRole(row)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}
func (r *Repository) HasActiveGlobalRole(ctx context.Context, userID identity.UserID, role identity.GlobalRole) (bool, error) {
	if _, err := identity.ParseGlobalRole(string(role)); err != nil {
		return false, err
	}
	key, err := uuid(string(userID))
	if err != nil {
		return false, err
	}
	found, err := r.q.HasActiveGlobalRole(ctx, sqlc.HasActiveGlobalRoleParams{UserID: key, Role: string(role)})
	return found, storageError(err)
}
func (r *Repository) RevokeGlobalRole(ctx context.Context, userID identity.UserID, role identity.GlobalRole) (identity.GlobalRoleAssignment, error) {
	if _, err := identity.ParseGlobalRole(string(role)); err != nil {
		return identity.GlobalRoleAssignment{}, err
	}
	key, err := uuid(string(userID))
	if err != nil {
		return identity.GlobalRoleAssignment{}, err
	}
	row, err := r.q.RevokeGlobalRole(ctx, sqlc.RevokeGlobalRoleParams{UserID: key, Role: string(role)})
	if err != nil {
		return identity.GlobalRoleAssignment{}, storageError(err)
	}
	return mapRole(row)
}
func (r *Repository) CreateSession(ctx context.Context, userID identity.UserID, digest identity.SessionTokenDigest, expiresAt time.Time) (identity.Session, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return identity.Session{}, err
	}
	row, err := r.q.CreateSession(ctx, sqlc.CreateSessionParams{UserID: key, TokenDigest: digest.Bytes(), ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}})
	if err != nil {
		return identity.Session{}, storageError(err)
	}
	return mapSession(row)
}
func (r *Repository) GetSessionByDigest(ctx context.Context, digest identity.SessionTokenDigest) (identity.Session, error) {
	row, err := r.q.GetSessionByDigest(ctx, digest.Bytes())
	if err != nil {
		return identity.Session{}, storageError(err)
	}
	return mapSession(row)
}
func (r *Repository) RevokeSession(ctx context.Context, id identity.SessionID) (identity.Session, error) {
	key, err := uuid(string(id))
	if err != nil {
		return identity.Session{}, err
	}
	row, err := r.q.RevokeSession(ctx, key)
	if err != nil {
		return identity.Session{}, storageError(err)
	}
	return mapSession(row)
}
func (r *Repository) RevokeUserSessions(ctx context.Context, userID identity.UserID) (int64, error) {
	key, err := uuid(string(userID))
	if err != nil {
		return 0, err
	}
	count, err := r.q.RevokeUserSessions(ctx, key)
	return count, storageError(err)
}
func (r *Repository) UpdateSessionLastSeen(ctx context.Context, id identity.SessionID) (identity.Session, error) {
	key, err := uuid(string(id))
	if err != nil {
		return identity.Session{}, err
	}
	row, err := r.q.UpdateSessionLastSeen(ctx, key)
	if err != nil {
		return identity.Session{}, storageError(err)
	}
	return mapSession(row)
}
