package audit

import (
	"context"
	"errors"
	"time"
)

type Action string

const (
	AdministratorBootstrapped   Action = "ADMINISTRATOR_BOOTSTRAPPED"
	LocalPasswordLoginCompleted Action = "LOCAL_PASSWORD_LOGIN_COMPLETED"
	SessionLogoutCompleted      Action = "SESSION_LOGOUT_COMPLETED"
)

type Event struct {
	ID                   string
	Action               Action
	ActorKind            string
	ActorUserID          string
	ResourceType         string
	ResourceID           string
	AuthenticationMethod string
	Outcome              string
	OperationID          string
	OccurredAt           time.Time
}

var ErrConflict = errors.New("audit event conflicts with existing data")
var ErrNotFound = errors.New("audit record not found")

type Repository interface {
	Append(context.Context, Event) (Event, error)
	ListForResource(context.Context, string, string) ([]Event, error)
	GetByOperationID(context.Context, string) (Event, error)
}
