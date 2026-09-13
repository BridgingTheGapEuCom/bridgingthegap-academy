package platform

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit"
	auditpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCompletedSessionTransactionUnavailable = errors.New("completed session transaction unavailable")

type postgresCompletedSessionTransactionRunner struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

type postgresCompletedSessionTransaction struct {
	sessions identity.SessionService
	audit    *auditpostgres.Repository
}

// NewPostgresLoginOrchestrator wires the existing Identity services and the
// narrowly scoped PostgreSQL transaction boundary at the composition root.
func NewPostgresLoginOrchestrator(pool *pgxpool.Pool, observer identity.PasswordAuthenticationObserver, now func() time.Time) LoginOrchestrator {
	if pool == nil {
		return NewLoginOrchestrator(nil, nil)
	}
	authenticator := identity.NewPasswordAuthenticator(identitypostgres.New(pool), identity.DefaultPasswordHasher(), observer, now)
	return NewLoginOrchestrator(authenticator, postgresCompletedSessionTransactionRunner{pool: pool, now: now})
}

func (r postgresCompletedSessionTransactionRunner) InCompletedSessionTransaction(ctx context.Context, fn func(CompletedSessionTransaction) error) error {
	if r.pool == nil || fn == nil {
		return ErrCompletedSessionTransactionUnavailable
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ErrCompletedSessionTransactionUnavailable
	}
	defer func() { _ = tx.Rollback(ctx) }()
	work := postgresCompletedSessionTransaction{
		sessions: identity.NewSessionService(identitypostgres.New(tx), nil, r.now),
		audit:    auditpostgres.New(tx),
	}
	if err := fn(work); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrCompletedSessionTransactionUnavailable
	}
	return nil
}

func (t postgresCompletedSessionTransaction) CreateSession(ctx context.Context, userID identity.UserID) (identity.CreatedSession, error) {
	return t.sessions.CreateSession(ctx, userID)
}

func (t postgresCompletedSessionTransaction) RevokeResolvedSession(ctx context.Context, current identity.ResolvedSession) error {
	return t.sessions.RevokeResolvedSession(ctx, current)
}

func (t postgresCompletedSessionTransaction) AppendAudit(ctx context.Context, event audit.Event) error {
	_, err := t.audit.Append(ctx, event)
	return err
}

func (t postgresCompletedSessionTransaction) GetAuditByOperationID(ctx context.Context, operationID string) (audit.Event, error) {
	return t.audit.GetByOperationID(ctx, operationID)
}
