//go:build integration

package platform

import (
	"context"
	"errors"
	"net/http"
	"sync"
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

func testAuthoringDraftMetadataMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-draft-metadata-mutation")
	if err != nil {
		t.Fatal(err)
	}
	identityRepository := identitypostgres.New(pool)
	user, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	authoringRepository := authoringpostgres.New(pool)
	draft, workspace, err := authoringRepository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	authorizer := authoring.NewAuthorizationService(authoringRepository)
	mutations := authoring.NewDraftMutationService(authoringRepository, authorizer)
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	created, err := sessions.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	readService := authoring.NewReadService(authoringRepository, authorizer)
	router := authTestRouter(&authHTTP{sessions: sessions, authoring: readService, authoringMutations: mutations})
	response := authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID), `{"expectedRevision":1,"title":"Writer B"}`, &http.Cookie{Name: sessionCookieName, Value: created.Token.Value()}, authTestCSRFToken().Value())
	if response.Code != http.StatusOK {
		t.Fatalf("initial PATCH = %d: %s", response.Code, response.Body.String())
	}
	resolved, err := sessions.ResolveSession(ctx, created.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	actor, err := identity.ActorFromResolvedSession(resolved)
	if err != nil {
		t.Fatal(err)
	}
	staleTitle := "Writer A"
	if _, err := mutations.UpdateDraft(ctx, actor, draft.ID, 1, authoring.DraftMetadataPatch{Title: &staleTitle}); !errors.Is(err, authoring.ErrRevisionMismatch) {
		t.Fatalf("stale PostgreSQL writer = %v", err)
	}
	stored, err := authoringRepository.GetDraft(ctx, draft.ID)
	if err != nil || stored.Metadata.Title != "Writer B" || stored.Metadata.Description != draft.Metadata.Description || stored.Revision != 2 || !stored.UpdatedAt.After(draft.UpdatedAt) {
		t.Fatalf("stale write changed committed draft: err=%v draft=%#v", err, stored)
	}
	if _, err := authoringRepository.RevokeMember(ctx, workspace.ID, string(user.ID)); err != nil {
		t.Fatal(err)
	}
	response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID), `{"expectedRevision":2,"title":"Revoked"}`, &http.Cookie{Name: sessionCookieName, Value: created.Token.Value()}, authTestCSRFToken().Value())
	if response.Code != http.StatusNotFound {
		t.Fatalf("revoked member PATCH = %d: %s", response.Code, response.Body.String())
	}
	if _, err := mutations.UpdateDraft(ctx, actor, draft.ID, stored.Revision, authoring.DraftMetadataPatch{Title: &staleTitle}); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("revoked member mutated draft: %v", err)
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
	response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID), `{"expectedRevision":2,"title":"Administrator"}`, &http.Cookie{Name: sessionCookieName, Value: adminSession.Token.Value()}, authTestCSRFToken().Value())
	if response.Code != http.StatusNotFound {
		t.Fatalf("global administrator bypassed authoring membership on PATCH = %d: %s", response.Code, response.Body.String())
	}

	// A separate real-PostgreSQL race confirms two callers with the same
	// revision cannot both commit or increment it twice.
	raceDraft, _, err := authoringRepository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, title := range []string{"Concurrent A", "Concurrent B"} {
		title := title
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := mutations.UpdateDraft(ctx, actor, raceDraft.ID, 1, authoring.DraftMetadataPatch{Title: &title})
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for result := range results {
		if result == nil {
			successes++
		} else if errors.Is(result, authoring.ErrRevisionMismatch) {
			conflicts++
		} else {
			t.Fatalf("concurrent mutation failed unexpectedly: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent mutations successes=%d conflicts=%d", successes, conflicts)
	}
	raceStored, err := authoringRepository.GetDraft(ctx, raceDraft.ID)
	if err != nil || raceStored.Revision != 2 {
		t.Fatalf("concurrent mutation revision = %d err=%v", raceStored.Revision, err)
	}
}
