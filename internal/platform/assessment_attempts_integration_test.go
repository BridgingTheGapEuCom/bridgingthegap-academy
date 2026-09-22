//go:build integration

package platform

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	assessmentspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAssessmentAttemptPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(courseRepository)
	course, err := courseRepository.CreateCourse(ctx, "assessment-attempt-context")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "99000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.1.0", "99000000-0000-4000-8000-000000000002"))
	if err != nil {
		t.Fatal(err)
	}
	for _, learner := range []string{"91000000-0000-4000-8000-000000000001", "91000000-0000-4000-8000-000000000002"} {
		if _, err := pool.Exec(ctx, "INSERT INTO identity.users (id) VALUES ($1)", learner); err != nil {
			t.Fatal(err)
		}
	}

	repository := assessmentspostgres.New(pool)
	responses := []assessments.AttemptResponse{
		{QuestionKey: "matching", Type: assessments.QuestionMatching, Pairs: []assessments.AttemptMatchingPair{{LeftItemKey: "left-a", RightItemKey: "right-one"}}},
		{QuestionKey: "multiple", Type: assessments.QuestionMultipleChoice, SelectedOptionKeys: []string{"one", "two"}},
		{QuestionKey: "single", Type: assessments.QuestionSingleChoice, SelectedOptionKey: "atomic"},
	}
	created, err := repository.CreateAssessmentAttempt(ctx, assessments.AttemptInput{
		LearnerUserID: "91000000-0000-4000-8000-000000000001", CourseVersionID: string(first.ID), AssessmentKey: "78000000-0000-4000-8000-000000000001", Responses: responses,
	})
	if err != nil || created.State != assessments.AttemptInProgress || created.Revision != 1 || created.SubmittedAt != nil {
		t.Fatalf("create attempt = %#v, %v", created, err)
	}
	loaded, err := repository.GetAssessmentAttempt(ctx, created.ID)
	if err != nil || !sameAttempt(created, loaded) {
		t.Fatalf("attempt round trip = %#v, %v", loaded, err)
	}
	if loaded.LearnerUserID != "91000000-0000-4000-8000-000000000001" || loaded.CourseVersionID != string(first.ID) || loaded.AssessmentKey != "78000000-0000-4000-8000-000000000001" {
		t.Fatalf("immutable attempt context was not preserved: %#v", loaded)
	}

	updated, err := repository.UpdateAssessmentAttempt(ctx, created.ID, created.Revision, assessments.AttemptUpdate{Responses: nil})
	if err != nil || updated.Revision != 2 || len(updated.Responses) != 0 {
		t.Fatalf("attempt update = %#v, %v", updated, err)
	}
	if _, err := repository.UpdateAssessmentAttempt(ctx, created.ID, created.Revision, assessments.AttemptUpdate{Responses: nil}); !errors.Is(err, assessments.ErrRevisionMismatch) {
		t.Fatalf("stale attempt update = %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := repository.UpdateAssessmentAttempt(ctx, created.ID, updated.Revision, assessments.AttemptUpdate{Responses: responses})
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	succeeded, stale := 0, 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, assessments.ErrRevisionMismatch) {
			stale++
		} else {
			t.Fatalf("concurrent attempt update = %v", err)
		}
	}
	if succeeded != 1 || stale != 1 {
		t.Fatalf("concurrent attempt updates success=%d stale=%d", succeeded, stale)
	}

	current, err := repository.GetAssessmentAttempt(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	submittedAt := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	submitted, err := repository.SubmitAssessmentAttempt(ctx, created.ID, current.Revision, submittedAt)
	if err != nil || submitted.State != assessments.AttemptSubmitted || submitted.SubmittedAt == nil || !submitted.SubmittedAt.Equal(submittedAt) {
		t.Fatalf("submit attempt = %#v, %v", submitted, err)
	}
	if _, err := repository.UpdateAssessmentAttempt(ctx, created.ID, submitted.Revision, assessments.AttemptUpdate{}); !errors.Is(err, assessments.ErrAttemptImmutable) {
		t.Fatalf("submitted update = %v", err)
	}

	other, err := repository.CreateAssessmentAttempt(ctx, assessments.AttemptInput{
		LearnerUserID: "91000000-0000-4000-8000-000000000002", CourseVersionID: string(second.ID), AssessmentKey: "78000000-0000-4000-8000-000000000001",
	})
	if err != nil || other.CourseVersionID == created.CourseVersionID || other.LearnerUserID == created.LearnerUserID {
		t.Fatalf("learner/version isolation = %#v, %v", other, err)
	}

	var beforeInvalid int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM assessments.assessment_attempt").Scan(&beforeInvalid); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateAssessmentAttempt(ctx, assessments.AttemptInput{
		LearnerUserID: "91000000-0000-4000-8000-000000000001", CourseVersionID: string(first.ID), AssessmentKey: "92000000-0000-4000-8000-000000000001",
	}); !errors.Is(err, assessments.ErrInvalidAttempt) {
		t.Fatalf("unbound assessment context = %v", err)
	}
	var afterInvalid int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM assessments.assessment_attempt").Scan(&afterInvalid); err != nil || afterInvalid != beforeInvalid {
		t.Fatalf("invalid context changed attempt count before=%d after=%d err=%v", beforeInvalid, afterInvalid, err)
	}
}

func sameAttempt(left, right assessments.AssessmentAttempt) bool {
	return left.ID == right.ID && left.LearnerUserID == right.LearnerUserID && left.CourseVersionID == right.CourseVersionID && left.AssessmentKey == right.AssessmentKey &&
		left.State == right.State && left.Revision == right.Revision && reflect.DeepEqual(left.Responses, right.Responses) && left.CreatedAt.Equal(right.CreatedAt) && left.UpdatedAt.Equal(right.UpdatedAt) &&
		(left.SubmittedAt == nil && right.SubmittedAt == nil || left.SubmittedAt != nil && right.SubmittedAt != nil && left.SubmittedAt.Equal(*right.SubmittedAt))
}
