package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository writes append-only Audit & Security records.
type Repository struct{ q *sqlc.Queries }

var _ audit.Repository = (*Repository)(nil)
var _ interface {
	RecordAdministratorBootstrap(context.Context, string, string) error
} = (*Repository)(nil)

func New(db sqlc.DBTX) *Repository { return &Repository{q: sqlc.New(db)} }

func (r *Repository) RecordAdministratorBootstrap(ctx context.Context, userID string, operationID string) error {
	_, err := r.Append(ctx, audit.Event{
		Action:       audit.AdministratorBootstrapped,
		ActorKind:    "SYSTEM",
		ResourceType: "IDENTITY_USER",
		ResourceID:   userID,
		Outcome:      "SUCCESS",
		OperationID:  operationID,
	})
	return err
}

func (r *Repository) Append(ctx context.Context, event audit.Event) (audit.Event, error) {
	resourceID, err := uuid(event.ResourceID)
	if err != nil {
		return audit.Event{}, errors.New("invalid audit resource identifier")
	}
	operationID, err := uuid(event.OperationID)
	if err != nil {
		return audit.Event{}, errors.New("invalid audit operation identifier")
	}
	var actorUserID pgtype.UUID
	if event.ActorUserID != "" {
		actorUserID, err = uuid(event.ActorUserID)
		if err != nil {
			return audit.Event{}, errors.New("invalid audit actor identifier")
		}
	}
	row, err := r.q.AppendEvent(ctx, sqlc.AppendEventParams{
		Action:               string(event.Action),
		ActorKind:            event.ActorKind,
		ActorUserID:          actorUserID,
		ResourceType:         event.ResourceType,
		ResourceID:           resourceID,
		AuthenticationMethod: pgtype.Text{String: event.AuthenticationMethod, Valid: event.AuthenticationMethod != ""},
		Outcome:              event.Outcome,
		OperationID:          operationID,
	})
	if err != nil {
		return audit.Event{}, storageError(err)
	}
	return mapEvent(row), nil
}

func (r *Repository) ListForResource(ctx context.Context, resourceType, resourceID string) ([]audit.Event, error) {
	id, err := uuid(resourceID)
	if err != nil {
		return nil, errors.New("invalid audit resource identifier")
	}
	rows, err := r.q.ListEventsForResource(ctx, sqlc.ListEventsForResourceParams{ResourceType: resourceType, ResourceID: id})
	if err != nil {
		return nil, storageError(err)
	}
	events := make([]audit.Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, mapEvent(row))
	}
	return events, nil
}

func (r *Repository) GetByOperationID(ctx context.Context, operationID string) (audit.Event, error) {
	id, err := uuid(operationID)
	if err != nil {
		return audit.Event{}, errors.New("invalid audit operation identifier")
	}
	row, err := r.q.GetEventByOperationID(ctx, id)
	if err != nil {
		return audit.Event{}, storageError(err)
	}
	return mapEvent(row), nil
}

func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, errors.New("invalid UUID")
	}
	return id, nil
}

func mapEvent(row sqlc.AuditEvent) audit.Event {
	var actorUserID string
	if row.ActorUserID.Valid {
		actorUserID = row.ActorUserID.String()
	}
	return audit.Event{ID: row.ID.String(), Action: audit.Action(row.Action), ActorKind: row.ActorKind, ActorUserID: actorUserID, ResourceType: row.ResourceType, ResourceID: row.ResourceID.String(), AuthenticationMethod: row.AuthenticationMethod.String, Outcome: row.Outcome, OperationID: row.OperationID.String(), OccurredAt: row.OccurredAt.Time}
}

func storageError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return audit.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return audit.ErrConflict
		}
		return fmt.Errorf("audit storage failure (SQLSTATE %s)", pgErr.Code)
	}
	return errors.New("audit storage failure")
}
