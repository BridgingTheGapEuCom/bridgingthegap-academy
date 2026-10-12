package progress

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type CourseVersionRepository interface {
	GetPublishedImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
}

type Service struct {
	progress Repository
	courses  CourseVersionRepository
}

func NewService(progress Repository, courseVersions CourseVersionRepository) *Service {
	return &Service{progress: progress, courses: courseVersions}
}

func (s *Service) Get(ctx context.Context, learner string, courseID courses.CourseID, version courses.Version) (CourseProgress, error) {
	aggregate, err := s.course(ctx, learner, courseID, version)
	if err != nil {
		return CourseProgress{}, err
	}
	value, err := s.progress.Get(ctx, learner, string(aggregate.ID))
	if errors.Is(err, ErrProgressNotFound) {
		return s.progress.Create(ctx, learner, string(aggregate.ID))
	}
	return value, err
}

func (s *Service) CompleteLesson(ctx context.Context, learner string, courseID courses.CourseID, version courses.Version, lessonKey string, expectedRevision int64) (CourseProgress, error) {
	aggregate, err := s.course(ctx, learner, courseID, version)
	if err != nil {
		return CourseProgress{}, err
	}
	found := false
	for _, module := range aggregate.Modules {
		for _, lesson := range module.Lessons {
			if lesson.StableKey == lessonKey {
				found = true
				break
			}
		}
	}
	if !found {
		return CourseProgress{}, ErrProgressNotFound
	}
	return s.progress.MarkLessonCompleted(ctx, learner, string(aggregate.ID), lessonKey, expectedRevision)
}

func (s *Service) course(ctx context.Context, learner string, courseID courses.CourseID, version courses.Version) (courses.ImmutableCourseVersion, error) {
	if s == nil || s.progress == nil || s.courses == nil || !validUUID(learner) || !version.Valid() {
		return courses.ImmutableCourseVersion{}, ErrProgressUnavailable
	}
	aggregate, err := s.courses.GetPublishedImmutableCourseVersionByCourseAndVersion(ctx, courseID, version)
	if err != nil {
		if errors.Is(err, courses.ErrNotFound) {
			return courses.ImmutableCourseVersion{}, ErrProgressNotFound
		}
		return courses.ImmutableCourseVersion{}, ErrProgressUnavailable
	}
	return aggregate, nil
}
