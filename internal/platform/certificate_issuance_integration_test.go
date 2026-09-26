//go:build integration

package platform

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	credentialspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress"
	progresspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testCertificateIssuance(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(courseRepository)
	course, err := courseRepository.CreateCourse(ctx, "certificate-issuance")
	if err != nil {
		t.Fatal(err)
	}
	v1, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "d1000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	v2, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.1.0", "d1000000-0000-4000-8000-000000000002"))
	if err != nil {
		t.Fatal(err)
	}
	identities := identitypostgres.New(pool)
	learner, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	ineligible, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	concurrent, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}

	progresses := progresspostgres.New(pool)
	completeAllLessons(t, ctx, progresses, string(learner.ID), v1)
	completeAllLessons(t, ctx, progresses, string(concurrent.ID), v1)
	service, err := credentials.NewCertificateIssuanceService(credentialspostgres.New(pool), courseRepository, progresses, credentials.Issuer{ID: "https://academy.example.com", Name: "Academy"}, func() time.Time { return time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}

	issued, err := service.Issue(ctx, string(learner.ID), course.ID, v1.CourseVersion.Version)
	if err != nil || issued.Certificate == nil || issued.Existing || issued.Certificate.CourseVersionID != string(v1.ID) || issued.Certificate.Achievement.CourseVersion != "1.0.0" || issued.Certificate.Issuer.Name != "Academy" {
		t.Fatalf("v1 issue = %#v, %v", issued, err)
	}
	loaded, err := credentialspostgres.New(pool).GetByID(ctx, issued.Certificate.ID)
	if err != nil || loaded.ID != issued.Certificate.ID || loaded.Achievement.Criteria != "Completed every published lesson in this CourseVersion." {
		t.Fatalf("stored issue = %#v, %v", loaded, err)
	}
	notEligible, err := service.Issue(ctx, string(ineligible.ID), course.ID, v1.CourseVersion.Version)
	if err != nil || notEligible.Certificate != nil || notEligible.Eligibility.Reason != credentials.EligibilityMissingProgress {
		t.Fatalf("missing progress issue = %#v, %v", notEligible, err)
	}
	v2Result, err := service.Issue(ctx, string(learner.ID), course.ID, v2.CourseVersion.Version)
	if err != nil || v2Result.Certificate != nil || v2Result.Eligibility.Reason != credentials.EligibilityMissingProgress {
		t.Fatalf("v2 isolation = %#v, %v", v2Result, err)
	}

	start := make(chan struct{})
	results := make(chan credentials.IssuanceResult, 2)
	errors := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			result, issueErr := service.Issue(ctx, string(concurrent.ID), course.ID, v1.CourseVersion.Version)
			results <- result
			errors <- issueErr
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(errors)
	var ids []credentials.CertificateID
	for result := range results {
		if result.Certificate == nil {
			t.Fatalf("concurrent issue returned no certificate: %#v", result)
		}
		ids = append(ids, result.Certificate.ID)
	}
	for issueErr := range errors {
		if issueErr != nil {
			t.Fatalf("concurrent issue error = %v", issueErr)
		}
	}
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatalf("concurrent certificate ids = %#v", ids)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credentials.certificate WHERE learner_user_id=$1 AND course_version_id=$2`, learnerID(concurrent.ID), string(v1.ID)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent persisted certificates = %d, %v", count, err)
	}
}

func completeAllLessons(t *testing.T, ctx context.Context, repository progress.Repository, learnerID string, version courses.ImmutableCourseVersion) {
	t.Helper()
	completed, err := repository.Create(ctx, learnerID, string(version.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range version.Modules {
		for _, lesson := range module.Lessons {
			completed, err = repository.MarkLessonCompleted(ctx, learnerID, string(version.ID), lesson.StableKey, completed.Revision)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func learnerID(id identity.UserID) string { return string(id) }
