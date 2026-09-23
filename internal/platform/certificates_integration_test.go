//go:build integration

package platform

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	credentialspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testCertificatePersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(courseRepository)
	course, err := courseRepository.CreateCourse(ctx, "certificate-foundation")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "c1000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.1.0", "c1000000-0000-4000-8000-000000000002"))
	if err != nil {
		t.Fatal(err)
	}
	otherCourse, err := courseRepository.CreateCourse(ctx, "certificate-other-course")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Store(ctx, immutableCourseVersionFixture(t, otherCourse.ID, "1.0.0", "c1000000-0000-4000-8000-000000000003")); err != nil {
		t.Fatal(err)
	}
	identities := identitypostgres.New(pool)
	learnerA, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	learnerB, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}

	repository := credentialspostgres.New(pool)
	issuedAt := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	input := certificateInput(t, learnerA.ID, first, "Completed every lesson in this published CourseVersion.")
	if err := input.Validate(); err != nil {
		t.Fatalf("certificate input = %#v, %v", input, err)
	}
	created, err := repository.Create(ctx, input, issuedAt)
	if err != nil || created.Status != credentials.CertificateActive || created.RevokedAt != nil || !created.IssuedAt.Equal(issuedAt) {
		t.Fatalf("create certificate = %#v, %v", created, err)
	}
	if created.CourseID != string(course.ID) || created.CourseVersionID != string(first.ID) || created.Achievement.CourseVersion != "1.0.0" {
		t.Fatalf("exact CourseVersion was not frozen: %#v", created)
	}
	loaded, err := repository.GetByID(ctx, created.ID)
	if err != nil || !reflect.DeepEqual(loaded, created) {
		t.Fatalf("certificate round trip = %#v, %v", loaded, err)
	}

	replayed, err := repository.Create(ctx, input, issuedAt.Add(time.Hour))
	if err != nil || replayed.ID != created.ID || !reflect.DeepEqual(replayed.Achievement, created.Achievement) {
		t.Fatalf("idempotent issuance = %#v, %v", replayed, err)
	}
	mismatchedContext := certificateInput(t, learnerB.ID, first, input.Achievement.Criteria)
	mismatchedContext.CourseID = string(otherCourse.ID)
	if _, err := repository.Create(ctx, mismatchedContext, issuedAt); !errors.Is(err, credentials.ErrInvalidCertificate) {
		t.Fatalf("mismatched CourseVersion context = %v", err)
	}
	otherLearner, err := repository.Create(ctx, certificateInput(t, learnerB.ID, first, input.Achievement.Criteria), issuedAt)
	if err != nil || otherLearner.ID == created.ID {
		t.Fatalf("cross-learner isolation = %#v, %v", otherLearner, err)
	}
	otherVersion, err := repository.Create(ctx, certificateInput(t, learnerA.ID, second, input.Achievement.Criteria), issuedAt)
	if err != nil || otherVersion.ID == created.ID || otherVersion.Achievement.CourseVersion != "1.1.0" {
		t.Fatalf("cross-version isolation = %#v, %v", otherVersion, err)
	}

	revoked, err := repository.Revoke(ctx, created.ID, issuedAt.Add(time.Hour))
	if err != nil || revoked.Status != credentials.CertificateRevoked || revoked.RevokedAt == nil || !reflect.DeepEqual(revoked.Achievement, created.Achievement) || !reflect.DeepEqual(revoked.Issuer, created.Issuer) {
		t.Fatalf("revoke = %#v, %v", revoked, err)
	}
	storedRevocation, err := repository.GetByLearnerAndCourseVersion(ctx, string(learnerA.ID), string(first.ID))
	if err != nil || !reflect.DeepEqual(storedRevocation, revoked) {
		t.Fatalf("stored revocation = %#v, %v", storedRevocation, err)
	}
}

func certificateInput(t *testing.T, learnerID identity.UserID, version courses.ImmutableCourseVersion, criteria string) credentials.CertificateInput {
	t.Helper()
	achievement, err := credentials.CourseCompletionAchievement(version, criteria)
	if err != nil {
		t.Fatal(err)
	}
	return credentials.CertificateInput{LearnerUserID: string(learnerID), CourseID: string(version.CourseVersion.CourseID), CourseVersionID: string(version.ID), Achievement: achievement, Issuer: credentials.Issuer{ID: "academy.example", Name: "Academy"}}
}
