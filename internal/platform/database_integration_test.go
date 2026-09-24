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
	t.Run("assets persistence", func(t *testing.T) { testAssetsPersistence(t, ctx, pool) })
	t.Run("assessments persistence", func(t *testing.T) { testAssessmentsPersistence(t, ctx, pool) })
	t.Run("authoring assessment API", func(t *testing.T) { testAuthoringAssessmentAPI(t, ctx, pool) })
	t.Run("authoring asset upload API", func(t *testing.T) { testAuthoringAssetUploadAPI(t, ctx, pool) })
	t.Run("courses persistence", func(t *testing.T) { testCoursesPersistence(t, ctx, pool) })
	t.Run("course structure persistence", func(t *testing.T) { testCourseStructurePersistence(t, ctx, pool) })
	t.Run("immutable course version publication persistence", func(t *testing.T) { testImmutableCourseVersionPersistence(t, ctx, pool) })
	t.Run("imported publication provenance persistence", func(t *testing.T) { testImportedPublicationProvenancePersistence(t, ctx, pool) })
	t.Run("caller-owned CourseVersion transaction persistence", func(t *testing.T) { testCallerOwnedCourseVersionTransactionPersistence(t, ctx, pool) })
	t.Run("translation persistence", func(t *testing.T) { testTranslationPersistence(t, ctx, pool) })
	t.Run("imported translation publication", func(t *testing.T) { testImportedTranslationPublication(t, ctx, pool) })
	t.Run("transactional Course package import", func(t *testing.T) { testPortabilityImport(t, ctx, pool) })
	t.Run("plugin registry and trust persistence", func(t *testing.T) { testPluginRegistry(t, ctx, pool) })
	t.Run("immutable published course version reads", func(t *testing.T) { testImmutablePublishedCourseVersionReads(t, ctx, pool) })
	t.Run("published course catalog", func(t *testing.T) { testPublishedCourseCatalog(t, ctx, pool) })
	t.Run("publication orchestration and recovery", func(t *testing.T) { testPublicationOrchestration(t, ctx, pool) })
	t.Run("publication asset resolution and immutable binding", func(t *testing.T) { testPublicationAssetBinding(t, ctx, pool) })
	t.Run("publication Assessment resolution and immutable binding", func(t *testing.T) { testPublicationAssessmentBinding(t, ctx, pool) })
	t.Run("course community persistence", func(t *testing.T) { testCourseCommunityPersistence(t, ctx, pool) })
	t.Run("course community API", func(t *testing.T) { testCourseCommunityAPI(t, ctx, pool) })
	t.Run("assessment attempt persistence", func(t *testing.T) { testAssessmentAttemptPersistence(t, ctx, pool) })
	t.Run("certificate persistence", func(t *testing.T) { testCertificatePersistence(t, ctx, pool) })
	t.Run("certificate issuance", func(t *testing.T) { testCertificateIssuance(t, ctx, pool) })
	t.Run("Open Badges status allocation and payload", func(t *testing.T) { testOpenBadgesStatus(t, ctx, pool) })
	t.Run("signed Open Badges publication", func(t *testing.T) { testSignedOpenBadgesPublication(t, ctx, pool) })
	t.Run("learner assessment attempt API", func(t *testing.T) { testLearnerAssessmentAttemptAPI(t, ctx, pool) })
	t.Run("published asset binary delivery", func(t *testing.T) { testPublishedAssetBinaryDelivery(t, ctx, pool) })
	t.Run("authoring publication API", func(t *testing.T) { testAuthoringPublicationAPI(t, ctx, pool) })
	t.Run("course public read service", func(t *testing.T) { testCourseReadService(t, ctx, pool) })
	t.Run("authoring persistence", func(t *testing.T) { testAuthoringPersistence(t, ctx, pool) })
	t.Run("authoring draft creation", func(t *testing.T) { testAuthoringDraftCreation(t, ctx, pool) })
	t.Run("authoring authorization", func(t *testing.T) { testAuthoringAuthorization(t, ctx, pool) })
	t.Run("authoring private read API", func(t *testing.T) { testAuthoringReadAPI(t, ctx, pool) })
	t.Run("authoring draft metadata mutation", func(t *testing.T) { testAuthoringDraftMetadataMutation(t, ctx, pool) })
	t.Run("authoring module mutation", func(t *testing.T) { testAuthoringModuleMutation(t, ctx, pool) })
	t.Run("authoring lesson mutation", func(t *testing.T) { testAuthoringLessonMutation(t, ctx, pool) })
	t.Run("authoring lesson content mutation", func(t *testing.T) { testAuthoringLessonContentMutation(t, ctx, pool) })
	t.Run("authoring API hardening", func(t *testing.T) { testAuthoringHardening(t, ctx, pool) })
	t.Run("authoring membership mutation", func(t *testing.T) { testAuthoringMembershipMutation(t, ctx, pool) })
	t.Run("authoring review persistence", func(t *testing.T) { testAuthoringReviewPersistence(t, ctx, pool) })
	t.Run("authoring review decision policy", func(t *testing.T) { testAuthoringReviewDecisionPolicy(t, ctx, pool) })
	t.Run("authoring review API", func(t *testing.T) { testAuthoringReviewAPI(t, ctx, pool) })
}
