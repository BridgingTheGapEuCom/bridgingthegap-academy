package community

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// PublishedCourseLookup is deliberately narrow: Community only needs to know
// that its durable Course identity has learner-servable published content.
type PublishedCourseLookup interface {
	ListPublishedCourseVersions(context.Context) ([]courses.CourseVersion, error)
}

// Service owns Course-scoped participation orchestration. It does not know
// roles: while no enrollment model exists, every authenticated actor may
// participate in an enabled community for a published Course.
type Service struct {
	repository Repository
	courses    PublishedCourseLookup
}

func NewService(repository Repository, publishedCourses PublishedCourseLookup) *Service {
	return &Service{repository: repository, courses: publishedCourses}
}

func (s *Service) GetCommunity(ctx context.Context, courseID string) (CourseCommunity, error) {
	if s == nil || s.repository == nil || s.courses == nil || !uuid(courseID) {
		return CourseCommunity{}, ErrNotFound
	}
	if err := s.requirePublishedCourse(ctx, courseID); err != nil {
		return CourseCommunity{}, err
	}
	return s.repository.GetCommunity(ctx, courseID)
}

func (s *Service) ListThreads(ctx context.Context, courseID string, limit, offset int) ([]ThreadSummary, int, error) {
	community, err := s.GetCommunity(ctx, courseID)
	if err != nil {
		return nil, 0, err
	}
	// Disabled is explicit in metadata but has no participant-visible content.
	if community.Mode == CommunityDisabled {
		return []ThreadSummary{}, 0, nil
	}
	return s.repository.ListVisibleThreads(ctx, courseID, limit, offset)
}

func (s *Service) GetThread(ctx context.Context, courseID, threadID string, limit, offset int) (Thread, []Post, int, error) {
	community, err := s.GetCommunity(ctx, courseID)
	if err != nil {
		return Thread{}, nil, 0, err
	}
	if community.Mode == CommunityDisabled {
		return Thread{}, nil, 0, ErrNotFound
	}
	thread, err := s.repository.GetVisibleThread(ctx, courseID, threadID)
	if err != nil {
		return Thread{}, nil, 0, err
	}
	posts, total, err := s.repository.ListVisiblePosts(ctx, threadID, limit, offset)
	return thread, posts, total, err
}

func (s *Service) CreateThread(ctx context.Context, input ThreadInput) (Thread, Post, error) {
	community, err := s.GetCommunity(ctx, input.CourseID)
	if err != nil {
		return Thread{}, Post{}, err
	}
	if community.Mode != CommunityEnabled {
		return Thread{}, Post{}, ErrDisabled
	}
	return s.repository.CreateThread(ctx, input)
}

func (s *Service) CreatePost(ctx context.Context, input PostInput) (Post, error) {
	community, err := s.GetCommunity(ctx, input.CourseID)
	if err != nil {
		return Post{}, err
	}
	if community.Mode != CommunityEnabled {
		return Post{}, ErrDisabled
	}
	if _, err := s.repository.GetVisibleThread(ctx, input.CourseID, input.ThreadID); err != nil {
		return Post{}, err
	}
	return s.repository.CreatePost(ctx, input)
}

func (s *Service) requirePublishedCourse(ctx context.Context, courseID string) error {
	versions, err := s.courses.ListPublishedCourseVersions(ctx)
	if err != nil {
		return err
	}
	for _, version := range versions {
		if string(version.CourseID) == courseID && version.Status == courses.CourseVersionPublished {
			return nil
		}
	}
	return ErrNotFound
}

func IsUnavailable(err error) bool { return errors.Is(err, ErrNotFound) }
