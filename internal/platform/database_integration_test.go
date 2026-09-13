//go:build integration

package platform

import (
	"context"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPostgreSQLMigrationAndSchemaCompatibility(t *testing.T) {
	ctx := context.Background()
	container, err := postgrescontainer.Run(ctx, "postgres:17-alpine",
		postgrescontainer.WithDatabase("btg_lms"), postgrescontainer.WithUsername("btg"), postgrescontainer.WithPassword("test_password"),
		postgrescontainer.BasicWaitStrategies(), postgrescontainer.WithSQLDriver("pgx"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := postgres.SchemaCheck(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Run("identity persistence", func(t *testing.T) { testIdentityPersistence(t, ctx, pool) })
	t.Run("administrator bootstrap", func(t *testing.T) { testAdministratorBootstrap(t, ctx, pool) })
	t.Run("password authentication", func(t *testing.T) { testPasswordAuthentication(t, ctx, pool) })
}
