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
	t.Run("session lifecycle", func(t *testing.T) { testSessionLifecycle(t, ctx, pool) })
	t.Run("completed login and logout", func(t *testing.T) { testCompletedLoginLogout(t, ctx, pool) })
	t.Run("HTTP session transport", func(t *testing.T) { testHTTPAuthTransport(t, ctx, pool) })
	t.Run("HTTP login admission", func(t *testing.T) { testHTTPLoginAdmission(t, ctx, pool) })
	t.Run("HTTP authorization boundary", func(t *testing.T) { testHTTPAuthorizationBoundary(t, ctx, pool) })
	t.Run("courses persistence", func(t *testing.T) { testCoursesPersistence(t, ctx, pool) })
	t.Run("course structure persistence", func(t *testing.T) { testCourseStructurePersistence(t, ctx, pool) })
	t.Run("course public read service", func(t *testing.T) { testCourseReadService(t, ctx, pool) })
	t.Run("authoring persistence", func(t *testing.T) { testAuthoringPersistence(t, ctx, pool) })
	t.Run("authoring authorization", func(t *testing.T) { testAuthoringAuthorization(t, ctx, pool) })
	t.Run("authoring private read API", func(t *testing.T) { testAuthoringReadAPI(t, ctx, pool) })
	t.Run("authoring draft metadata mutation", func(t *testing.T) { testAuthoringDraftMetadataMutation(t, ctx, pool) })
	t.Run("authoring module mutation", func(t *testing.T) { testAuthoringModuleMutation(t, ctx, pool) })
	t.Run("authoring lesson mutation", func(t *testing.T) { testAuthoringLessonMutation(t, ctx, pool) })
	t.Run("authoring lesson content mutation", func(t *testing.T) { testAuthoringLessonContentMutation(t, ctx, pool) })
	t.Run("authoring API hardening", func(t *testing.T) { testAuthoringHardening(t, ctx, pool) })
	t.Run("authoring membership mutation", func(t *testing.T) { testAuthoringMembershipMutation(t, ctx, pool) })
}
