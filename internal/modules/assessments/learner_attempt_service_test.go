package assessments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type attemptRepositoryFake struct {
	attempts map[AttemptID]AssessmentAttempt
	next     int
}

func (r *attemptRepositoryFake) CreateAssessmentAttempt(_ context.Context, input AttemptInput) (AssessmentAttempt, error) {
	if err := input.Validate(); err != nil {
		return AssessmentAttempt{}, err
	}
	r.next++
	id := AttemptID("70000000-0000-4000-8000-00000000000" + string(rune('0'+r.next)))
	attempt := AssessmentAttempt{ID: id, LearnerUserID: input.LearnerUserID, CourseVersionID: input.CourseVersionID, AssessmentKey: input.AssessmentKey, State: AttemptInProgress, Revision: 1, Responses: input.Responses, CreatedAt: testAttemptTime, UpdatedAt: testAttemptTime}
	r.attempts[id] = attempt
	return attempt, nil
}

func (r *attemptRepositoryFake) GetAssessmentAttempt(_ context.Context, id AttemptID) (AssessmentAttempt, error) {
	attempt, ok := r.attempts[id]
	if !ok {
		return AssessmentAttempt{}, ErrAttemptNotFound
	}
	return attempt, nil
}

func (r *attemptRepositoryFake) UpdateAssessmentAttempt(_ context.Context, id AttemptID, expected int64, update AttemptUpdate) (AssessmentAttempt, error) {
	attempt, err := r.GetAssessmentAttempt(context.Background(), id)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	if attempt.State != AttemptInProgress {
		return AssessmentAttempt{}, ErrAttemptImmutable
	}
	if attempt.Revision != expected {
		return AssessmentAttempt{}, ErrRevisionMismatch
	}
	updated, err := attempt.WithResponses(update.Responses, testAttemptTime.Add(time.Minute))
	if err != nil {
		return AssessmentAttempt{}, err
	}
	r.attempts[id] = updated
	return updated, nil
}

func (r *attemptRepositoryFake) SubmitAssessmentAttempt(_ context.Context, id AttemptID, expected int64, result AttemptResult, at time.Time) (AssessmentAttempt, error) {
	attempt, err := r.GetAssessmentAttempt(context.Background(), id)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	if attempt.State != AttemptInProgress {
		return AssessmentAttempt{}, ErrAttemptImmutable
	}
	if attempt.Revision != expected {
		return AssessmentAttempt{}, ErrRevisionMismatch
	}
	submitted, err := attempt.Submit(result, at)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	r.attempts[id] = submitted
	return submitted, nil
}

type attemptCourseRepositoryFake struct {
	published courses.ImmutableCourseVersion
	immutable courses.ImmutableCourseVersion
	err       error
}

func (r attemptCourseRepositoryFake) GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error) {
	if r.err != nil {
		return courses.ImmutableCourseVersion{}, r.err
	}
	return r.published, nil
}

func (r attemptCourseRepositoryFake) GetImmutableCourseVersion(context.Context, courses.CourseVersionID) (courses.ImmutableCourseVersion, error) {
	if r.err != nil {
		return courses.ImmutableCourseVersion{}, r.err
	}
	return r.immutable, nil
}

func TestLearnerAttemptServiceUsesFrozenBindingAndOwnership(t *testing.T) {
	version, err := courses.ParseVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	aggregate := courses.ImmutableCourseVersion{ID: testCourseVersionID, AssessmentBindings: []courses.PublishedAssessmentBinding{gradingBinding()}}
	attempts := &attemptRepositoryFake{attempts: map[AttemptID]AssessmentAttempt{}}
	service := NewLearnerAttemptService(attempts, attemptCourseRepositoryFake{published: aggregate, immutable: aggregate}, func() time.Time { return testAttemptTime.Add(time.Hour) })
	attempt, err := service.Start(context.Background(), testLearnerID, "80000000-0000-4000-8000-000000000001", version, testAssessmentID)
	if err != nil || attempt.State != AttemptInProgress || attempt.CourseVersionID != testCourseVersionID || attempt.LearnerUserID != testLearnerID {
		t.Fatalf("start = %#v, %v", attempt, err)
	}
	if _, err := service.Update(context.Background(), "40000000-0000-4000-8000-000000000002", attempt.ID, attempt.Revision, completeCorrectResponses()); !errors.Is(err, ErrAttemptNotFound) {
		t.Fatalf("foreign update = %v", err)
	}
	updated, err := service.Update(context.Background(), testLearnerID, attempt.ID, attempt.Revision, completeCorrectResponses())
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	submitted, err := service.Submit(context.Background(), testLearnerID, attempt.ID, updated.Revision)
	if err != nil || submitted.State != AttemptSubmitted || submitted.Result == nil || *submitted.Result != (AttemptResult{CorrectCount: 3, TotalCount: 3}) {
		t.Fatalf("submit = %#v, %v", submitted, err)
	}
	if _, err := service.Submit(context.Background(), testLearnerID, attempt.ID, submitted.Revision); !errors.Is(err, ErrAttemptImmutable) {
		t.Fatalf("repeat submit = %v", err)
	}
}
