//go:build integration

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community"
	communitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func testCourseCommunityPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	cr := coursespostgres.New(pool)
	c, err := cr.CreateCourse(ctx, "community-persistence")
	if err != nil {
		t.Fatal(err)
	}
	u, err := identitypostgres.New(pool).CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	r := communitypostgres.New(pool)
	if _, err = r.CreateCommunity(ctx, string(c.ID), community.CommunityEnabled); err != nil {
		t.Fatal(err)
	}
	thread, opening, err := r.CreateThread(ctx, community.ThreadInput{CourseID: string(c.ID), Title: " Course question ", CreatedByUserID: string(u.ID), OpeningBody: "First line\nsecond line"})
	if err != nil || thread.Title != "Course question" || opening.ThreadID != thread.ID || opening.Body != "First line\nsecond line" {
		t.Fatalf("thread/opening=%#v %#v %v", thread, opening, err)
	}
	if got, err := r.GetPost(ctx, opening.ID); err != nil || got.AuthorUserID != string(u.ID) {
		t.Fatalf("post=%#v %v", got, err)
	}
	reply, err := r.CreatePost(ctx, community.PostInput{CourseID: string(c.ID), ThreadID: thread.ID, AuthorUserID: string(u.ID), Body: "Reply"})
	if err != nil || reply.ThreadID != thread.ID || reply.Body != "Reply" {
		t.Fatalf("reply=%#v %v", reply, err)
	}
	var replies sync.WaitGroup
	errs := make(chan error, 2)
	for _, body := range []string{"Concurrent one", "Concurrent two"} {
		replies.Add(1)
		go func(body string) {
			defer replies.Done()
			_, err := r.CreatePost(ctx, community.PostInput{CourseID: string(c.ID), ThreadID: thread.ID, AuthorUserID: string(u.ID), Body: body})
			errs <- err
		}(body)
	}
	replies.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	threads, total, err := r.ListVisibleThreads(ctx, string(c.ID), 20, 0)
	if err != nil || total != 1 || len(threads) != 1 || threads[0].PostCount != 4 || threads[0].ID != thread.ID {
		t.Fatalf("threads=%#v total=%d err=%v", threads, total, err)
	}
	posts, total, err := r.ListVisiblePosts(ctx, thread.ID, 20, 0)
	if err != nil || total != 4 || len(posts) != 4 || posts[0].ID != opening.ID || posts[1].ID != reply.ID {
		t.Fatalf("posts=%#v total=%d err=%v", posts, total, err)
	}
	if _, err := r.GetVisibleThread(ctx, string(c.ID), thread.ID); err != nil {
		t.Fatal(err)
	}
	other, err := cr.CreateCourse(ctx, "community-other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateCommunity(ctx, string(other.ID), community.CommunityEnabled); err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.CreateThread(ctx, community.ThreadInput{CourseID: string(other.ID), Title: "x", CreatedByUserID: string(u.ID), OpeningBody: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetVisibleThread(ctx, string(other.ID), thread.ID); !errors.Is(err, community.ErrNotFound) {
		t.Fatalf("foreign thread read was not hidden: %v", err)
	}
}

func testCourseCommunityAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	coursesRepository := coursespostgres.New(pool)
	course, err := coursesRepository.CreateCourse(ctx, "community-api")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = courses.NewCourseVersionStore(coursesRepository).Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "f1000000-0000-4000-8000-000000000001")); err != nil {
		t.Fatal(err)
	}
	communityRepository := communitypostgres.New(pool)
	if _, err = communityRepository.CreateCommunity(ctx, string(course.ID), community.CommunityEnabled); err != nil {
		t.Fatal(err)
	}
	identities := identitypostgres.New(pool)
	user, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identities, nil, nil)
	session, err := sessions.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	router := authTestRouter(&authHTTP{sessions: sessions, community: community.NewService(communityRepository, coursesRepository)})
	base := "/api/courses/by-id/" + string(course.ID) + "/community"
	created := authRequest(router, http.MethodPost, base+"/threads", `{"title":"Question","body":"Opening"}`, &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}, authTestCSRFToken().Value())
	if created.Code != http.StatusCreated || created.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	listed := authRequest(router, http.MethodGet, base+"/threads", "", &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()})
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), "Question") || strings.Contains(listed.Body.String(), "state") {
		t.Fatalf("list=%d %s", listed.Code, listed.Body.String())
	}
	var out struct {
		ThreadID string `json:"threadId"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &out); err != nil || out.ThreadID == "" {
		t.Fatalf("thread response=%s err=%v", created.Body.String(), err)
	}
	reply := authRequest(router, http.MethodPost, base+"/threads/"+out.ThreadID+"/posts", `{"body":"Reply"}`, &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}, authTestCSRFToken().Value())
	if reply.Code != http.StatusCreated || strings.Contains(reply.Body.String(), "state") {
		t.Fatalf("reply=%d %s", reply.Code, reply.Body.String())
	}
}
