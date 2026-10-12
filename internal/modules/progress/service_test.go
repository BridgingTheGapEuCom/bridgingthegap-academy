package progress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type serviceProgressRepository struct{ value CourseProgress }

func (r *serviceProgressRepository) Create(_ context.Context, learner, version string) (CourseProgress, error) {
	r.value, _ = New(learner, version, time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC))
	return r.value, nil
}
func (r *serviceProgressRepository) Get(context.Context, string, string) (CourseProgress, error) {
	if r.value.Revision == 0 {
		return CourseProgress{}, ErrProgressNotFound
	}
	return r.value, nil
}
func (r *serviceProgressRepository) MarkLessonCompleted(_ context.Context, learner, version, key string, revision int64) (CourseProgress, error) {
	if r.value.Revision != revision {
		return CourseProgress{}, ErrRevisionMismatch
	}
	next, _, err := r.value.MarkLessonCompleted(key, time.Date(2026, 10, 11, 10, 1, 0, 0, time.UTC))
	if err == nil {
		r.value = next
	}
	return next, err
}

type serviceCourseRepository struct {
	value courses.ImmutableCourseVersion
}

func (r serviceCourseRepository) GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error) {
	return r.value, nil
}

func TestServiceCreatesFirstUseAndCompletesOnlyPublishedLessons(t *testing.T) {
	version, _ := courses.ParseVersion("1.0.0")
	repository := &serviceProgressRepository{}
	service := NewService(repository, serviceCourseRepository{value: courses.ImmutableCourseVersion{
		ID:      "20000000-0000-4000-8000-000000000001",
		Modules: []courses.ImmutableCourseVersionModule{{Lessons: []courses.ImmutableCourseVersionLesson{{StableKey: "lesson-one"}}}},
	}})
	learner := "10000000-0000-4000-8000-000000000001"
	value, err := service.Get(context.Background(), learner, "30000000-0000-4000-8000-000000000001", version)
	if err != nil || value.Revision != 1 || len(value.CompletedLessonKeys) != 0 {
		t.Fatalf("first use = %#v, %v", value, err)
	}
	value, err = service.CompleteLesson(context.Background(), learner, "30000000-0000-4000-8000-000000000001", version, "lesson-one", value.Revision)
	if err != nil || value.Revision != 2 || len(value.CompletedLessonKeys) != 1 {
		t.Fatalf("complete = %#v, %v", value, err)
	}
	if _, err := service.CompleteLesson(context.Background(), learner, "30000000-0000-4000-8000-000000000001", version, "missing", value.Revision); !errors.Is(err, ErrProgressNotFound) {
		t.Fatalf("missing lesson = %v", err)
	}
}
