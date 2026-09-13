package platform

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/google/uuid"
)

var ErrLoginUnavailable = errors.New("login unavailable")
var ErrLogoutUnavailable = errors.New("logout unavailable")

type LocalPasswordAuthenticator interface {
	AuthenticatePassword(context.Context, string, []byte) (identity.AuthenticatedIdentity, error)
}

// CompletedSessionTransaction is deliberately scoped to session mutation and
// its Audit append. The PostgreSQL implementation uses one database transaction.
type CompletedSessionTransaction interface {
	CreateSession(context.Context, identity.UserID) (identity.CreatedSession, error)
	RevokeResolvedSession(context.Context, identity.ResolvedSession) (bool, error)
	AppendAudit(context.Context, audit.Event) error
	GetAuditByOperationID(context.Context, string) (audit.Event, error)
}

type CompletedSessionTransactionRunner interface {
	InCompletedSessionTransaction(context.Context, func(CompletedSessionTransaction) error) error
}

type LoginResult struct {
	UserID          identity.UserID
	SessionID       identity.SessionID
	Token           identity.RawSessionToken
	ExpiresAt       time.Time
	Method          identity.AuthenticationMethod
	AuthenticatedAt time.Time
	OperationID     string
}

type LoginOrchestrator struct {
	authenticator LocalPasswordAuthenticator
	transactions  CompletedSessionTransactionRunner
}

func NewLoginOrchestrator(authenticator LocalPasswordAuthenticator, transactions CompletedSessionTransactionRunner) LoginOrchestrator {
	return LoginOrchestrator{authenticator: authenticator, transactions: transactions}
}

// LoginWithPassword returns a bearer token only after the session and Audit
// event have committed together. Authentication failures never enter the
// transaction. The caller's password slice is cleared before return.
func (o LoginOrchestrator) LoginWithPassword(ctx context.Context, email string, password []byte, operationID string) (LoginResult, error) {
	defer clear(password)
	if o.authenticator == nil || o.transactions == nil {
		return LoginResult{}, ErrLoginUnavailable
	}
	operationID, err := completedOperationID(operationID)
	if err != nil {
		return LoginResult{}, ErrLoginUnavailable
	}
	actor, err := o.authenticator.AuthenticatePassword(ctx, email, password)
	if err != nil {
		return LoginResult{}, err
	}
	if actor.UserID == "" || actor.Method != identity.AuthenticationMethodLocalPassword || actor.AuthenticatedAt.IsZero() {
		return LoginResult{}, ErrLoginUnavailable
	}
	var created identity.CreatedSession
	err = o.transactions.InCompletedSessionTransaction(ctx, func(tx CompletedSessionTransaction) error {
		var createErr error
		created, createErr = tx.CreateSession(ctx, actor.UserID)
		if createErr != nil {
			return createErr
		}
		if created.SessionID == "" || created.UserID != actor.UserID || created.Token.Value() == "" {
			return ErrLoginUnavailable
		}
		return tx.AppendAudit(ctx, audit.Event{
			Action:               audit.LocalPasswordLoginCompleted,
			ActorKind:            "USER",
			ActorUserID:          string(actor.UserID),
			ResourceType:         "IDENTITY_SESSION",
			ResourceID:           string(created.SessionID),
			AuthenticationMethod: string(actor.Method),
			Outcome:              "SUCCESS",
			OperationID:          operationID,
		})
	})
	if err != nil {
		return LoginResult{}, ErrLoginUnavailable
	}
	return LoginResult{UserID: actor.UserID, SessionID: created.SessionID, Token: created.Token, ExpiresAt: created.ExpiresAt, Method: actor.Method, AuthenticatedAt: actor.AuthenticatedAt, OperationID: operationID}, nil
}

// Logout requires a trusted current session returned by session resolution.
// Authorization of that context belongs to the later transport boundary.
// A retry with the same operation ID reuses its completed Audit fact only if
// the user and session match. Already-revoked sessions create no new fact.
func (o LoginOrchestrator) Logout(ctx context.Context, current identity.ResolvedSession, operationID string) error {
	if o.transactions == nil {
		return ErrLogoutUnavailable
	}
	if current.SessionID() == "" || current.UserID() == "" {
		return identity.ErrInvalidSession
	}
	operationID, err := completedOperationID(operationID)
	if err != nil {
		return ErrLogoutUnavailable
	}
	err = o.transactions.InCompletedSessionTransaction(ctx, func(tx CompletedSessionTransaction) error {
		changed, err := tx.RevokeResolvedSession(ctx, current)
		if err != nil {
			return err
		}
		existing, err := tx.GetAuditByOperationID(ctx, operationID)
		if err == nil {
			if existing.Action == audit.SessionLogoutCompleted && existing.ActorUserID == string(current.UserID()) && existing.ResourceType == "IDENTITY_SESSION" && existing.ResourceID == string(current.SessionID()) && existing.Outcome == "SUCCESS" {
				return nil
			}
			return ErrLogoutUnavailable
		}
		if !errors.Is(err, audit.ErrNotFound) {
			return err
		}
		if !changed {
			return nil
		}
		return tx.AppendAudit(ctx, audit.Event{
			Action:       audit.SessionLogoutCompleted,
			ActorKind:    "USER",
			ActorUserID:  string(current.UserID()),
			ResourceType: "IDENTITY_SESSION",
			ResourceID:   string(current.SessionID()),
			Outcome:      "SUCCESS",
			OperationID:  operationID,
		})
	})
	if err != nil {
		if errors.Is(err, identity.ErrInvalidSession) {
			return identity.ErrInvalidSession
		}
		return ErrLogoutUnavailable
	}
	return nil
}

// Operation IDs follow the UUID convention used by Audit and administrator
// bootstrap. A future transport can pass its existing correlation UUID.
func completedOperationID(id string) (string, error) {
	if id == "" {
		generated, err := uuid.NewRandom()
		if err != nil {
			return "", err
		}
		return generated.String(), nil
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}
