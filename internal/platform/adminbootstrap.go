package platform

import (
	"context"
	"errors"
	"time"

	auditpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/audit/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAdministratorBootstrapUnavailable = errors.New("administrator bootstrap unavailable")

type AdministratorBootstrapOptions struct {
	Hasher      identity.PasswordHasher
	Clock       func() time.Time
	OperationID string
}

func BootstrapAdministrator(ctx context.Context, pool *pgxpool.Pool, email string, password []byte, options AdministratorBootstrapOptions) (identity.User, error) {
	defer clear(password)
	if pool == nil {
		return identity.User{}, ErrAdministratorBootstrapUnavailable
	}
	operationID := options.OperationID
	if operationID == "" {
		generated, err := uuid.NewRandom()
		if err != nil {
			return identity.User{}, ErrAdministratorBootstrapUnavailable
		}
		operationID = generated.String()
	}
	hasher := options.Hasher
	if hasher == nil {
		hasher = identity.DefaultPasswordHasher()
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return identity.User{}, ErrAdministratorBootstrapUnavailable
	}
	defer func() { _ = tx.Rollback(ctx) }()
	service := identity.NewAdministratorBootstrapper(hasher, options.Clock)
	user, err := service.Bootstrap(ctx, identitypostgres.New(tx), auditpostgres.New(tx), identity.AdministratorBootstrapInput{
		Email:       email,
		Password:    password,
		OperationID: operationID,
	})
	if err != nil {
		return identity.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return identity.User{}, ErrAdministratorBootstrapUnavailable
	}
	return user, nil
}
