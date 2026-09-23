package assessments

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// AttemptCourseVersionRepository is the narrow Courses-owned read boundary
// required by learner attempts. It resolves only frozen CourseVersion
// aggregates; mutable Authoring Assessments are never consulted.
type AttemptCourseVersionRepository interface {
	GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
	GetImmutableCourseVersion(context.Context, courses.CourseVersionID) (courses.ImmutableCourseVersion, error)
}

// LearnerAttemptService coordinates private learner ownership with immutable
// published assessment bindings. It has no HTTP or session dependency.
type LearnerAttemptService struct {
	attempts AttemptRepository
	courses  AttemptCourseVersionRepository
	now      func() time.Time
}

func NewLearnerAttemptService(attempts AttemptRepository, courseVersions AttemptCourseVersionRepository, now func() time.Time) *LearnerAttemptService {
	if now == nil {
		now = time.Now
	}
	return &LearnerAttemptService{attempts: attempts, courses: courseVersions, now: now}
}

func (s *LearnerAttemptService) Start(ctx context.Context, learnerUserID string, courseID courses.CourseID, version courses.Version, assessmentKey AssessmentID) (AssessmentAttempt, error) {
	if s == nil || s.attempts == nil || s.courses == nil || !validUUID(learnerUserID) || !validUUID(string(assessmentKey)) || !version.Valid() {
		return AssessmentAttempt{}, ErrAttemptUnavailable
	}
	aggregate, err := s.courses.GetPublishedImmutableCourseVersionByCourseAndVersion(ctx, courseID, version)
	if err != nil {
		return AssessmentAttempt{}, attemptContextError(err)
	}
	if _, ok := publishedAssessmentBinding(aggregate, assessmentKey); !ok {
		return AssessmentAttempt{}, ErrAttemptUnavailable
	}
	return s.attempts.CreateAssessmentAttempt(ctx, AttemptInput{LearnerUserID: learnerUserID, CourseVersionID: string(aggregate.ID), AssessmentKey: assessmentKey})
}

func (s *LearnerAttemptService) Get(ctx context.Context, learnerUserID string, id AttemptID) (AssessmentAttempt, error) {
	attempt, err := s.owned(ctx, learnerUserID, id)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	return attempt, nil
}

func (s *LearnerAttemptService) Update(ctx context.Context, learnerUserID string, id AttemptID, expectedRevision int64, responses []AttemptResponse) (AssessmentAttempt, error) {
	attempt, err := s.owned(ctx, learnerUserID, id)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	binding, err := s.bindingForAttempt(ctx, attempt)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	canonical, err := ValidateResponsesForPublishedAssessment(binding, responses, false)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	return s.attempts.UpdateAssessmentAttempt(ctx, id, expectedRevision, AttemptUpdate{Responses: canonical})
}

func (s *LearnerAttemptService) Submit(ctx context.Context, learnerUserID string, id AttemptID, expectedRevision int64) (AssessmentAttempt, error) {
	attempt, err := s.owned(ctx, learnerUserID, id)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	binding, err := s.bindingForAttempt(ctx, attempt)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	result, err := GradePublishedAssessment(binding, attempt.Responses)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	return s.attempts.SubmitAssessmentAttempt(ctx, id, expectedRevision, result, s.now().UTC())
}

func (s *LearnerAttemptService) owned(ctx context.Context, learnerUserID string, id AttemptID) (AssessmentAttempt, error) {
	if s == nil || s.attempts == nil || !validUUID(learnerUserID) {
		return AssessmentAttempt{}, ErrAttemptUnavailable
	}
	attempt, err := s.attempts.GetAssessmentAttempt(ctx, id)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	if attempt.LearnerUserID != learnerUserID {
		return AssessmentAttempt{}, ErrAttemptNotFound
	}
	return attempt, nil
}

func (s *LearnerAttemptService) bindingForAttempt(ctx context.Context, attempt AssessmentAttempt) (courses.PublishedAssessmentBinding, error) {
	if s == nil || s.courses == nil {
		return courses.PublishedAssessmentBinding{}, ErrAttemptUnavailable
	}
	aggregate, err := s.courses.GetImmutableCourseVersion(ctx, courses.CourseVersionID(attempt.CourseVersionID))
	if err != nil {
		return courses.PublishedAssessmentBinding{}, attemptContextError(err)
	}
	binding, ok := publishedAssessmentBinding(aggregate, attempt.AssessmentKey)
	if !ok {
		return courses.PublishedAssessmentBinding{}, ErrAttemptUnavailable
	}
	return binding, nil
}

func publishedAssessmentBinding(aggregate courses.ImmutableCourseVersion, key AssessmentID) (courses.PublishedAssessmentBinding, bool) {
	for _, binding := range aggregate.AssessmentBindings {
		if binding.AssessmentKey == string(key) && binding.Validate() == nil {
			return binding, true
		}
	}
	return courses.PublishedAssessmentBinding{}, false
}

func attemptContextError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ErrAttemptUnavailable
}
