//go:build integration

package platform

import (
	"context"
	"net/http"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringReadAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-private-read-api")
	if err != nil {
		t.Fatal(err)
	}
	authoringRepository := authoringpostgres.New(pool)
	creator := "99999999-9999-4999-8999-999999999999"
	draftA, workspaceA, err := authoringRepository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), creator)
	if err != nil {
		t.Fatal(err)
	}
	draftB, _, err := authoringRepository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	moduleA, err := authoringRepository.CreateModule(ctx, authoring.ModuleInput{DraftID: draftA.ID, StableKey: "basics", Title: "Basics", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	moduleB, err := authoringRepository.CreateModule(ctx, authoring.ModuleInput{DraftID: draftB.ID, StableKey: "basics", Title: "Basics", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	lessonA, err := authoringRepository.CreateLesson(ctx, lessonFixture(draftA.ID, moduleA.ID, "first-lesson", 0))
	if err != nil {
		t.Fatal(err)
	}
	lessonB, err := authoringRepository.CreateLesson(ctx, lessonFixture(draftB.ID, moduleB.ID, "other-lesson", 0))
	if err != nil {
		t.Fatal(err)
	}
	identityRepository := identitypostgres.New(pool)
	member, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authoringRepository.AddMember(ctx, workspaceA.ID, string(member.ID), authoring.MemberAuthor); err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	created, err := sessions.CreateSession(ctx, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	readService := authoring.NewReadService(authoringRepository, authoring.NewAuthorizationService(authoringRepository))
	router := authTestRouter(&authHTTP{sessions: sessions, authoring: readService})
	cookie := &http.Cookie{Name: sessionCookieName, Value: created.Token.Value()}

	for _, path := range []string{
		"/api/authoring/drafts/" + string(draftA.ID),
		"/api/authoring/drafts/" + string(draftA.ID) + "/workspace",
		"/api/authoring/drafts/" + string(draftA.ID) + "/structure",
		"/api/authoring/drafts/" + string(draftA.ID) + "/lessons/" + string(lessonA.ID),
	} {
		response := authRequest(router, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s = %d cache=%q: %s", path, response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
	}
	if response := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftA.ID)+"/lessons/"+string(lessonB.ID), "", cookie); response.Code != http.StatusNotFound {
		t.Fatalf("cross-draft lesson read = %d: %s", response.Code, response.Body.String())
	}
	if response := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftB.ID), "", cookie); response.Code != http.StatusNotFound {
		t.Fatalf("other draft read = %d: %s", response.Code, response.Body.String())
	}
	if _, err := authoringRepository.RevokeMember(ctx, workspaceA.ID, string(member.ID)); err != nil {
		t.Fatal(err)
	}
	if response := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftA.ID), "", cookie); response.Code != http.StatusNotFound {
		t.Fatalf("revoked member read = %d: %s", response.Code, response.Body.String())
	}

	administrator, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := identityRepository.AssignGlobalRole(ctx, administrator.ID, identity.RoleAdministrator, nil); err != nil {
		t.Fatal(err)
	}
	adminSession, err := sessions.CreateSession(ctx, administrator.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftA.ID), "", &http.Cookie{Name: sessionCookieName, Value: adminSession.Token.Value()}); response.Code != http.StatusNotFound {
		t.Fatalf("global administrator bypassed authoring membership = %d: %s", response.Code, response.Body.String())
	}
}
