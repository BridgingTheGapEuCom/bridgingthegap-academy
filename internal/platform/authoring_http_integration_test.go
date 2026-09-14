//go:build integration

package platform

import (
	"context"
	"errors"
	"net/http"
	"strconv"
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

func testAuthoringModuleMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-module-mutation")
	if err != nil {
		t.Fatal(err)
	}
	identityRepository := identitypostgres.New(pool)
	user, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	repository := authoringpostgres.New(pool)
	draft, workspace, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	authorizer := authoring.NewAuthorizationService(repository)
	mutations := authoring.NewModuleMutationService(repository, authorizer)
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	session, err := sessions.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	router := authTestRouter(&authHTTP{sessions: sessions, authoringStructureMutations: mutations})
	cookie := &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}
	csrf := authTestCSRFToken().Value()
	base := "/api/authoring/drafts/" + string(draft.ID) + "/modules"

	response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":1,"stableKey":"advanced","title":"Advanced","position":0}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("first module create = %d: %s", response.Code, response.Body.String())
	}
	response = authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":2,"stableKey":"basics","title":"Basics","position":0}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("middle module create = %d: %s", response.Code, response.Body.String())
	}
	modules, err := repository.ListModules(ctx, draft.ID)
	if err != nil || len(modules) != 2 || modules[0].StableKey != "basics" || modules[0].Position != 0 || modules[1].StableKey != "advanced" || modules[1].Position != 1 {
		t.Fatalf("insert did not shift positions: %v %#v", err, modules)
	}
	basics, advanced := modules[0], modules[1]
	response = authRequest(router, http.MethodPatch, base+"/"+string(basics.ID), `{"expectedModuleRevision":1,"title":"Fundamentals"}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("module update = %d: %s", response.Code, response.Body.String())
	}
	basics, err = repository.GetModule(ctx, basics.ID)
	if err != nil || basics.Revision != 2 || basics.StableKey != "basics" {
		t.Fatalf("metadata update altered key/revision: %v %#v", err, basics)
	}
	draft, err = repository.GetDraft(ctx, draft.ID)
	if err != nil || draft.Revision != 4 {
		t.Fatalf("draft revision after update = %d err=%v", draft.Revision, err)
	}
	response = authRequest(router, http.MethodPut, base+"/order", `{"expectedDraftRevision":4,"moduleIds":["`+string(advanced.ID)+`","`+string(basics.ID)+`"]}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("module reorder = %d: %s", response.Code, response.Body.String())
	}
	modules, err = repository.ListModules(ctx, draft.ID)
	if err != nil || modules[0].ID != advanced.ID || modules[0].Position != 0 || modules[1].ID != basics.ID || modules[1].Position != 1 {
		t.Fatalf("reorder was not contiguous: %v %#v", err, modules)
	}
	if response = authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4,"stableKey":"stale","title":"Stale","position":2}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("stale create = %d: %s", response.Code, response.Body.String())
	}
	basics, err = repository.GetModule(ctx, basics.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateLesson(ctx, lessonFixture(draft.ID, basics.ID, "module-lesson", 0)); err != nil {
		t.Fatal(err)
	}
	draft, err = repository.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	response = authRequest(router, http.MethodDelete, base+"/"+string(basics.ID), `{"expectedDraftRevision":`+strconv.FormatInt(draft.Revision, 10)+`,"expectedModuleRevision":`+strconv.FormatInt(basics.Revision, 10)+`}`, cookie, csrf)
	if response.Code != http.StatusConflict {
		t.Fatalf("non-empty module delete = %d: %s", response.Code, response.Body.String())
	}
	advanced, err = repository.GetModule(ctx, advanced.ID)
	if err != nil {
		t.Fatal(err)
	}
	response = authRequest(router, http.MethodDelete, base+"/"+string(advanced.ID), `{"expectedDraftRevision":`+strconv.FormatInt(draft.Revision, 10)+`,"expectedModuleRevision":`+strconv.FormatInt(advanced.Revision, 10)+`}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("empty module delete = %d: %s", response.Code, response.Body.String())
	}
	modules, err = repository.ListModules(ctx, draft.ID)
	if err != nil || len(modules) != 1 || modules[0].ID != basics.ID || modules[0].Position != 0 {
		t.Fatalf("delete did not compact positions: %v %#v", err, modules)
	}

	other, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	otherModule, _, err := repository.CreateModuleAtPosition(ctx, other.ID, other.Revision, authoring.ModuleInput{DraftID: other.ID, StableKey: "other", Title: "Other", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	draft, err = repository.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	response = authRequest(router, http.MethodPatch, base+"/"+string(otherModule.ID), `{"expectedModuleRevision":1,"title":"Leak"}`, cookie, csrf)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-draft module mutation = %d: %s", response.Code, response.Body.String())
	}
	if _, err := repository.RevokeMember(ctx, workspace.ID, string(user.ID)); err != nil {
		t.Fatal(err)
	}
	response = authRequest(router, http.MethodPatch, base+"/"+string(basics.ID), `{"expectedModuleRevision":`+strconv.FormatInt(basics.Revision, 10)+`,"title":"Revoked"}`, cookie, csrf)
	if response.Code != http.StatusNotFound {
		t.Fatalf("revoked structural mutation = %d", response.Code)
	}

	// Independent PostgreSQL callers with the same aggregate revision cannot
	// both commit a full reorder.
	raceDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	raceOne, _, err := repository.CreateModuleAtPosition(ctx, raceDraft.ID, 1, authoring.ModuleInput{DraftID: raceDraft.ID, StableKey: "one", Title: "One", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	raceTwo, raceCurrent, err := repository.CreateModuleAtPosition(ctx, raceDraft.ID, 2, authoring.ModuleInput{DraftID: raceDraft.ID, StableKey: "two", Title: "Two", Position: 1})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, order := range [][]authoring.ModuleID{{raceOne.ID, raceTwo.ID}, {raceTwo.ID, raceOne.ID}} {
		order := order
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := repository.ReorderModules(ctx, raceDraft.ID, raceCurrent.Revision, order)
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, authoring.ErrRevisionMismatch) {
			conflicts++
		} else {
			t.Fatalf("concurrent reorder error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent reorder results successes=%d conflicts=%d", successes, conflicts)
	}
	raced, err := repository.ListModules(ctx, raceDraft.ID)
	if err != nil || len(raced) != 2 || raced[0].Position != 0 || raced[1].Position != 1 || raced[0].ID == raced[1].ID {
		t.Fatalf("concurrent reorder corrupted positions: %v %#v", err, raced)
	}
}
