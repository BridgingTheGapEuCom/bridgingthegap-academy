package platform

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

const communityTestCourse = "11111111-1111-4111-8111-111111111111"
const communityTestThread = "22222222-2222-4222-8222-222222222222"
const communityTestPost = "33333333-3333-4333-8333-333333333333"

type communityCourseFake struct{ versions []courses.CourseVersion }

func (f communityCourseFake) ListPublishedCourseVersions(context.Context) ([]courses.CourseVersion, error) {
	return f.versions, nil
}

type communityRepositoryFake struct {
	mode        community.Mode
	threads     []community.ThreadSummary
	posts       []community.Post
	createInput community.ThreadInput
	postInput   community.PostInput
}

func (f *communityRepositoryFake) CreateCommunity(context.Context, string, community.Mode) (community.CourseCommunity, error) {
	return community.CourseCommunity{}, errors.New("unexpected")
}
func (f *communityRepositoryFake) GetCommunity(_ context.Context, course string) (community.CourseCommunity, error) {
	if course != communityTestCourse {
		return community.CourseCommunity{}, community.ErrNotFound
	}
	return community.CourseCommunity{CourseID: course, Mode: f.mode, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (f *communityRepositoryFake) CreateThread(_ context.Context, in community.ThreadInput) (community.Thread, community.Post, error) {
	f.createInput = in
	now := time.Now()
	return community.Thread{ID: communityTestThread, CourseID: in.CourseID, Title: strings.TrimSpace(in.Title), CreatedByUserID: in.CreatedByUserID, State: community.Visible, CreatedAt: now, UpdatedAt: now}, community.Post{ID: communityTestPost, ThreadID: communityTestThread, AuthorUserID: in.CreatedByUserID, Body: strings.TrimSpace(in.OpeningBody), State: community.Visible, CreatedAt: now, UpdatedAt: now}, nil
}
func (f *communityRepositoryFake) GetThread(context.Context, string) (community.Thread, error) {
	return community.Thread{}, community.ErrNotFound
}
func (f *communityRepositoryFake) GetPost(context.Context, string) (community.Post, error) {
	return community.Post{}, community.ErrNotFound
}
func (f *communityRepositoryFake) ListVisibleThreads(context.Context, string, int, int) ([]community.ThreadSummary, int, error) {
	return f.threads, len(f.threads), nil
}
func (f *communityRepositoryFake) GetVisibleThread(_ context.Context, course, id string) (community.Thread, error) {
	if course != communityTestCourse || id != communityTestThread {
		return community.Thread{}, community.ErrNotFound
	}
	now := time.Now()
	return community.Thread{ID: id, CourseID: course, Title: "Thread", CreatedByUserID: "44444444-4444-4444-8444-444444444444", State: community.Visible, CreatedAt: now, UpdatedAt: now}, nil
}
func (f *communityRepositoryFake) ListVisiblePosts(context.Context, string, int, int) ([]community.Post, int, error) {
	return f.posts, len(f.posts), nil
}
func (f *communityRepositoryFake) CreatePost(_ context.Context, in community.PostInput) (community.Post, error) {
	f.postInput = in
	now := time.Now()
	return community.Post{ID: communityTestPost, ThreadID: in.ThreadID, AuthorUserID: in.AuthorUserID, Body: strings.TrimSpace(in.Body), State: community.Visible, CreatedAt: now, UpdatedAt: now}, nil
}
func (f *communityRepositoryFake) SetThreadState(_ context.Context, course, id string, state community.Visibility) (community.Thread, error) {
	return community.Thread{ID: id, CourseID: course, Title: "Thread", CreatedByUserID: "44444444-4444-4444-8444-444444444444", State: state, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (f *communityRepositoryFake) SetPostState(_ context.Context, _, thread, id string, state community.Visibility) (community.Post, error) {
	return community.Post{ID: id, ThreadID: thread, AuthorUserID: "44444444-4444-4444-8444-444444444444", Body: "Post", State: state, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func communityHTTP(t *testing.T, repo *communityRepositoryFake, session identity.ResolvedSession) http.Handler {
	t.Helper()
	service := community.NewService(repo, communityCourseFake{versions: []courses.CourseVersion{{CourseID: courses.CourseID(communityTestCourse), Status: courses.CourseVersionPublished}}})
	return authTestRouter(&authHTTP{sessions: &authResolverFake{current: session}, community: service})
}
func communitySession(t *testing.T) identity.ResolvedSession { t.Helper(); return loginTestCurrent(t) }
func communityCookie() *http.Cookie {
	return &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
}

func TestCommunityHTTPCreatesThreadFromTrustedRouteAndSession(t *testing.T) {
	repo := &communityRepositoryFake{mode: community.CommunityEnabled}
	router := communityHTTP(t, repo, communitySession(t))
	response := authRequest(router, http.MethodPost, "/api/courses/by-id/"+communityTestCourse+"/community/threads", `{"title":" Topic ","body":" First post "}`, communityCookie(), authTestCSRFToken().Value())
	if response.Code != http.StatusCreated || repo.createInput.CourseID != communityTestCourse || repo.createInput.CreatedByUserID != string(loginTestUser) || repo.createInput.Title != " Topic " || !strings.Contains(response.Body.String(), `"postId"`) || strings.Contains(response.Body.String(), `"state"`) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected create response %d %s", response.Code, response.Body.String())
	}
	bad := authRequest(router, http.MethodPost, "/api/courses/by-id/"+communityTestCourse+"/community/threads", `{"title":"x","body":"y","authorId":"bad"}`, communityCookie(), authTestCSRFToken().Value())
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("server-owned field was accepted: %d", bad.Code)
	}
}
func TestCommunityHTTPRequiresAuthenticationAndEnforcesDisabledMode(t *testing.T) {
	repo := &communityRepositoryFake{mode: community.CommunityDisabled}
	router := communityHTTP(t, repo, communitySession(t))
	unauth := authRequest(router, http.MethodPost, "/api/courses/by-id/"+communityTestCourse+"/community/threads", `{"title":"x","body":"y"}`, nil, authTestCSRFToken().Value())
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated write = %d", unauth.Code)
	}
	disabled := authRequest(router, http.MethodPost, "/api/courses/by-id/"+communityTestCourse+"/community/threads", `{"title":"x","body":"y"}`, communityCookie(), authTestCSRFToken().Value())
	if disabled.Code != http.StatusConflict || !strings.Contains(disabled.Body.String(), "community_disabled") {
		t.Fatalf("disabled write = %d %s", disabled.Code, disabled.Body.String())
	}
	list := authRequest(router, http.MethodGet, "/api/courses/by-id/"+communityTestCourse+"/community/threads", "", communityCookie())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"threads":[]`) {
		t.Fatalf("disabled community list = %d %s", list.Code, list.Body.String())
	}
}
func TestCommunityHTTPUsesCourseScopedVisibleReadAndPagination(t *testing.T) {
	now := time.Now()
	repo := &communityRepositoryFake{mode: community.CommunityEnabled, threads: []community.ThreadSummary{{Thread: community.Thread{ID: communityTestThread, CourseID: communityTestCourse, Title: "Visible", CreatedByUserID: "44444444-4444-4444-8444-444444444444", State: community.Visible, CreatedAt: now, UpdatedAt: now}, PostCount: 1}}, posts: []community.Post{{ID: communityTestPost, ThreadID: communityTestThread, AuthorUserID: "44444444-4444-4444-8444-444444444444", Body: "Visible post", State: community.Visible, CreatedAt: now, UpdatedAt: now}}}
	router := communityHTTP(t, repo, communitySession(t))
	list := authRequest(router, http.MethodGet, "/api/courses/by-id/"+communityTestCourse+"/community/threads?limit=1&offset=0", "", communityCookie())
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"postCount":1`) || strings.Contains(list.Body.String(), `"state"`) {
		t.Fatalf("unsafe list %d %s", list.Code, list.Body.String())
	}
	foreign := authRequest(router, http.MethodGet, "/api/courses/by-id/55555555-5555-4555-8555-555555555555/community/threads/"+communityTestThread, "", communityCookie())
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign thread = %d", foreign.Code)
	}
	invalid := authRequest(router, http.MethodGet, "/api/courses/by-id/"+communityTestCourse+"/community/threads?limit=101", "", communityCookie())
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unbounded page accepted: %d", invalid.Code)
	}
}
