package community

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

const (
	serviceCourseA = "10000000-0000-4000-8000-000000000001"
	serviceCourseB = "10000000-0000-4000-8000-000000000002"
	serviceActor   = "20000000-0000-4000-8000-000000000001"
	serviceThreadB = "30000000-0000-4000-8000-000000000002"
	servicePostB   = "40000000-0000-4000-8000-000000000002"
)

type serviceCourses []courses.CourseVersion

func (c serviceCourses) ListPublishedCourseVersions(context.Context) ([]courses.CourseVersion, error) {
	return c, nil
}

type serviceRepository struct{ visiblePostLookups int }

func (r *serviceRepository) CreateCommunity(context.Context, string, Mode) (CourseCommunity, error) {
	return CourseCommunity{}, ErrNotFound
}
func (r *serviceRepository) GetCommunity(context.Context, string) (CourseCommunity, error) {
	return CourseCommunity{}, ErrNotFound
}
func (r *serviceRepository) CreateThread(context.Context, ThreadInput) (Thread, Post, error) {
	return Thread{}, Post{}, ErrNotFound
}
func (r *serviceRepository) GetThread(context.Context, string) (Thread, error) {
	return Thread{}, ErrNotFound
}
func (r *serviceRepository) GetPost(context.Context, string) (Post, error) {
	return Post{}, ErrNotFound
}
func (r *serviceRepository) ListVisibleThreads(context.Context, string, int, int) ([]ThreadSummary, int, error) {
	return nil, 0, ErrNotFound
}
func (r *serviceRepository) GetVisibleThread(context.Context, string, string) (Thread, error) {
	return Thread{}, ErrNotFound
}
func (r *serviceRepository) ListVisiblePosts(context.Context, string, int, int) ([]Post, int, error) {
	r.visiblePostLookups++
	return []Post{{ID: servicePostB, ThreadID: serviceThreadB, State: Visible}}, 1, nil
}
func (r *serviceRepository) CreatePost(context.Context, PostInput) (Post, error) {
	return Post{}, ErrNotFound
}
func (r *serviceRepository) SetThreadState(context.Context, string, string, Visibility) (Thread, error) {
	return Thread{}, ErrNotFound
}
func (r *serviceRepository) SetPostState(context.Context, string, string, string, Visibility) (Post, error) {
	return Post{}, ErrNotFound
}
func (r *serviceRepository) ListThreadsForModeration(context.Context, string, int, int) ([]ThreadSummary, int, error) {
	return nil, 0, ErrNotFound
}
func (r *serviceRepository) GetThreadForModeration(_ context.Context, course, thread string) (Thread, error) {
	if course == serviceCourseA && thread == serviceThreadB {
		return Thread{}, ErrNotFound
	}
	return Thread{}, ErrNotFound
}
func (r *serviceRepository) ListPostsForModeration(context.Context, string, int, int) ([]Post, int, error) {
	return nil, 0, ErrNotFound
}

func TestModeratePostHidesForeignThreadExistenceBeforeOpeningPostCheck(t *testing.T) {
	repository := &serviceRepository{}
	service := NewService(repository, serviceCourses{{
		CourseID: serviceCourseA,
		Status:   courses.CourseVersionPublished,
		Attribution: []courses.ContributorSnapshot{{
			UserID: serviceActor,
			Role:   courses.ContributorAuthor,
		}},
		CreatedAt: time.Now(),
	}})

	_, err := service.ModeratePost(context.Background(), serviceCourseA, serviceActor, serviceThreadB, servicePostB, Hidden)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign Post moderation error = %v, want hidden not found", err)
	}
	if repository.visiblePostLookups != 0 {
		t.Fatalf("foreign Thread Posts were inspected %d times", repository.visiblePostLookups)
	}
}
