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

// Moderate uses immutable published attribution as the durable Course policy:
// a current authenticated contributor recorded as AUTHOR or MAINTAINER may
// moderate. Instance administrators receive no implicit bypass.
func (s *Service) ModerateThread(ctx context.Context, courseID, actorID, threadID string, state Visibility) (Thread, error) {
	if err := s.requireModerator(ctx, courseID, actorID); err != nil {
		return Thread{}, err
	}
	return s.repository.SetThreadState(ctx, courseID, threadID, state)
}
func (s *Service) ModeratePost(ctx context.Context, courseID, actorID, threadID, postID string, state Visibility) (Post, error) {
	if err := s.requireModerator(ctx, courseID, actorID); err != nil {
		return Post{}, err
	}
	if state == Hidden {
		posts, _, err := s.repository.ListVisiblePosts(ctx, threadID, 1, 0)
		if err != nil {
			return Post{}, err
		}
		if len(posts) > 0 && posts[0].ID == postID {
			return Post{}, ErrOpeningPost
		}
	}
	return s.repository.SetPostState(ctx, courseID, threadID, postID, state)
}
func (s *Service) CanModerate(ctx context.Context, courseID, actorID string) (bool, error) {
	err := s.requireModerator(ctx, courseID, actorID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
func (s *Service) ModeratorThreads(ctx context.Context, course, actor string, limit, offset int) ([]ThreadSummary, int, error) {
	if err := s.requireModerator(ctx, course, actor); err != nil {
		return nil, 0, err
	}
	return s.repository.ListThreadsForModeration(ctx, course, limit, offset)
}
func (s *Service) ModeratorThread(ctx context.Context, course, actor, thread string, limit, offset int) (Thread, []Post, int, error) {
	if err := s.requireModerator(ctx, course, actor); err != nil {
		return Thread{}, nil, 0, err
	}
	t, err := s.repository.GetThreadForModeration(ctx, course, thread)
	if err != nil {
		return Thread{}, nil, 0, err
	}
	p, n, err := s.repository.ListPostsForModeration(ctx, thread, limit, offset)
	return t, p, n, err
}
func (s *Service) requireModerator(ctx context.Context, courseID, actorID string) error {
	if !uuid(courseID) || !uuid(actorID) {
		return ErrNotFound
	}
	versions, err := s.courses.ListPublishedCourseVersions(ctx)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if string(v.CourseID) != courseID || v.Status != courses.CourseVersionPublished {
			continue
		}
		for _, c := range v.Attribution {
			if c.UserID == actorID && (c.Role == courses.ContributorAuthor || c.Role == courses.ContributorMaintainer) {
				return nil
			}
		}
	}
	return ErrNotFound
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
