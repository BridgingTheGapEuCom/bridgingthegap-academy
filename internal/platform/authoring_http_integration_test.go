//go:build integration

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, workspaceA.ID, string(member.ID), authoring.MemberAuthor); err != nil {
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
	if _, err := revokeTestAuthoringMember(ctx, pool, authoringRepository, workspaceA.ID, string(member.ID)); err != nil {
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
	// Revocation fixtures retain another maintainer, as production writes require.
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, workspace.ID, "77777777-7777-4777-8777-777777777777", authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	if _, err := revokeTestAuthoringMember(ctx, pool, authoringRepository, workspace.ID, string(user.ID)); err != nil {
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
	// Revocation fixtures retain another maintainer, as production writes require.
	if _, err := addTestAuthoringMember(ctx, pool, repository, workspace.ID, "77777777-7777-4777-8777-777777777777", authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	if _, err := revokeTestAuthoringMember(ctx, pool, repository, workspace.ID, string(user.ID)); err != nil {
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

func testAuthoringLessonMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-lesson-mutation")
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
	moduleA, draft, err := repository.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "first-module", Title: "First", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	moduleB, draft, err := repository.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "second-module", Title: "Second", Position: 1})
	if err != nil {
		t.Fatal(err)
	}
	authorizer := authoring.NewAuthorizationService(repository)
	mutations := authoring.NewLessonMutationService(repository, authorizer)
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	session, err := sessions.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorUser, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := addTestAuthoringMember(ctx, pool, repository, workspace.ID, string(authorUser.ID), authoring.MemberAuthor); err != nil {
		t.Fatal(err)
	}
	authorSession, err := sessions.CreateSession(ctx, authorUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	router := authTestRouter(&authHTTP{sessions: sessions, authoringLessonMutations: mutations})
	cookie := &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}
	authorCookie := &http.Cookie{Name: sessionCookieName, Value: authorSession.Token.Value()}
	csrf := authTestCSRFToken().Value()
	createPath := "/api/authoring/drafts/" + string(draft.ID) + "/modules/" + string(moduleA.ID) + "/lessons"
	draft, err = repository.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	createBody := `{"expectedDraftRevision":` + strconv.FormatInt(draft.Revision, 10) + `,"stableKey":"protected-lesson","title":"Protected","description":"A lesson.","objectives":["Understand"],"position":0}`
	if response := authRequest(router, http.MethodPost, createPath, createBody, nil, csrf); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated lesson create = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, createPath, createBody, cookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF lesson create = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, createPath, createBody, cookie, "wrong"); response.Code != http.StatusForbidden {
		t.Fatalf("wrong CSRF lesson create = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, createPath, `{"expectedDraftRevision":`+strconv.FormatInt(draft.Revision, 10)+`,"stableKey":"bad-input","title":"Bad","description":"A lesson.","objectives":["Understand"],"position":0,"content":{}}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("LessonContent create boundary = %d", response.Code)
	}
	create := func(module authoring.ModuleID, revision int64, key string, position int) authoring.DraftLesson {
		t.Helper()
		response := authRequest(router, http.MethodPost, "/api/authoring/drafts/"+string(draft.ID)+"/modules/"+string(module)+"/lessons", `{"expectedDraftRevision":`+strconv.FormatInt(revision, 10)+`,"stableKey":"`+key+`","title":"`+key+`","description":"A lesson.","objectives":["Understand"],"estimatedDurationMinutes":10,"position":`+strconv.Itoa(position)+`}`, cookie, csrf)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("create %s = %d: %s", key, response.Code, response.Body.String())
		}
		var dto struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &dto); err != nil || dto.ID == "" {
			t.Fatalf("lesson create DTO: %v %s", err, response.Body.String())
		}
		lesson, err := repository.GetLesson(ctx, authoring.LessonID(dto.ID))
		if err != nil {
			t.Fatal(err)
		}
		return lesson
	}
	first := create(moduleA.ID, draft.Revision, "first-lesson", 0)
	draft, _ = repository.GetDraft(ctx, draft.ID)
	second := create(moduleA.ID, draft.Revision, "second-lesson", 1)
	draft, _ = repository.GetDraft(ctx, draft.ID)
	third := create(moduleB.ID, draft.Revision, "third-lesson", 0)
	draft, _ = repository.GetDraft(ctx, draft.ID)
	lessons, err := repository.ListLessons(ctx, moduleA.ID)
	if err != nil || len(lessons) != 2 || lessons[0].ID != first.ID || lessons[1].ID != second.ID {
		t.Fatalf("lesson insertion order: %v %#v", err, lessons)
	}
	// An AUTHOR receives the same structural-edit capability as the creator
	// MAINTAINER, without a handler ever reading a membership role.
	response := authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(third.ID), `{"expectedLessonRevision":1,"description":"Author update"}`, authorCookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("AUTHOR lesson metadata PATCH = %d: %s", response.Code, response.Body.String())
	}
	draft, _ = repository.GetDraft(ctx, draft.ID)
	if response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(first.ID), `{"expectedLessonRevision":1,"stableKey":"renamed"}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("stable-key rename PATCH = %d", response.Code)
	}
	if response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(first.ID), `{"expectedLessonRevision":1,"content":{"schemaVersion":1,"blocks":[]}}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("LessonContent PATCH boundary = %d", response.Code)
	}
	if response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(first.ID), `{"expectedLessonRevision":1}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("empty lesson PATCH = %d", response.Code)
	}

	response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(first.ID), `{"expectedLessonRevision":1,"title":"First revised","objectives":["First","Second"],"estimatedDurationMinutes":null}`, cookie, csrf)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"content"`) {
		t.Fatalf("lesson metadata PATCH = %d: %s", response.Code, response.Body.String())
	}
	first, err = repository.GetLesson(ctx, first.ID)
	if err != nil || first.Title != "First revised" || first.EstimatedDurationMinutes != nil || first.StableKey != "first-lesson" || first.Revision != 2 {
		t.Fatalf("metadata patch changed identity/content: %v %#v", err, first)
	}
	draft, _ = repository.GetDraft(ctx, draft.ID)
	if response = authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(second.ID)+"/prerequisites", `{"expectedLessonRevision":1,"prerequisiteLessonKeys":["unknown-lesson"]}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("unknown prerequisite = %d", response.Code)
	}
	response = authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(second.ID)+"/prerequisites", `{"expectedLessonRevision":1,"prerequisiteLessonKeys":["first-lesson"]}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("prerequisite replacement = %d: %s", response.Code, response.Body.String())
	}
	if response = authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(second.ID)+"/prerequisites", `{"expectedLessonRevision":2,"prerequisiteLessonKeys":["second-lesson"]}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("self prerequisite = %d", response.Code)
	}
	if response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(second.ID), `{"expectedLessonRevision":1,"title":"Stale"}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("metadata after prerequisite mutation = %d", response.Code)
	}
	draft, _ = repository.GetDraft(ctx, draft.ID)
	response = authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/order", `{"expectedDraftRevision":`+strconv.FormatInt(draft.Revision, 10)+`,"modules":[{"moduleId":"`+string(moduleA.ID)+`","lessonIds":["`+string(second.ID)+`"]},{"moduleId":"`+string(moduleB.ID)+`","lessonIds":["`+string(first.ID)+`","`+string(third.ID)+`"]}]}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("cross-module reorder = %d: %s", response.Code, response.Body.String())
	}
	first, _ = repository.GetLesson(ctx, first.ID)
	if first.ModuleID != moduleB.ID || first.Position != 0 || first.StableKey != "first-lesson" || first.Revision != 3 {
		t.Fatalf("move lost lesson identity: %#v", first)
	}
	draft, _ = repository.GetDraft(ctx, draft.ID)
	if response = authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/order", `{"expectedDraftRevision":`+strconv.FormatInt(draft.Revision, 10)+`,"modules":[{"moduleId":"55555555-5555-4555-8555-555555555555","lessonIds":[]}]}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("foreign module reorder = %d", response.Code)
	}
	if response = authRequest(router, http.MethodPost, "/api/authoring/drafts/"+string(draft.ID)+"/modules/"+string(moduleB.ID)+"/lessons", `{"expectedDraftRevision":`+strconv.FormatInt(draft.Revision-1, 10)+`,"stableKey":"stale","title":"Stale","description":"A lesson.","objectives":["Understand"],"position":2}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("stale create after move = %d", response.Code)
	}
	second, _ = repository.GetLesson(ctx, second.ID)
	response = authRequest(router, http.MethodDelete, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(first.ID), `{"expectedDraftRevision":`+strconv.FormatInt(draft.Revision, 10)+`,"expectedLessonRevision":`+strconv.FormatInt(first.Revision, 10)+`}`, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("lesson delete = %d: %s", response.Code, response.Body.String())
	}
	if _, err := repository.GetLesson(ctx, first.ID); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("deleted lesson still readable: %v", err)
	}
	prerequisites, err := repository.ListPrerequisites(ctx, second.ID)
	second, _ = repository.GetLesson(ctx, second.ID)
	third, _ = repository.GetLesson(ctx, third.ID)
	if err != nil || len(prerequisites) != 0 || second.Revision != 4 || third.Position != 0 {
		t.Fatalf("incoming prerequisites or positions not cleaned: %v %#v %#v", err, prerequisites, second)
	}

	other, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	otherModule, other, err := repository.CreateModuleAtPosition(ctx, other.ID, other.Revision, authoring.ModuleInput{DraftID: other.ID, StableKey: "other-module", Title: "Other", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	otherLesson, _, err := repository.CreateLessonAtPosition(ctx, other.ID, otherModule.ID, other.Revision, authoring.NewDraftLessonInput(other.ID, otherModule.ID, "other-lesson", "Other", "A lesson.", []string{"Understand"}, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	if response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(otherLesson.ID), `{"expectedLessonRevision":1,"title":"Leak"}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("cross-draft lesson update = %d", response.Code)
	}

	raceDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	raceModuleA, raceDraft, err := repository.CreateModuleAtPosition(ctx, raceDraft.ID, raceDraft.Revision, authoring.ModuleInput{DraftID: raceDraft.ID, StableKey: "race-a", Title: "A", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	raceModuleB, raceDraft, err := repository.CreateModuleAtPosition(ctx, raceDraft.ID, raceDraft.Revision, authoring.ModuleInput{DraftID: raceDraft.ID, StableKey: "race-b", Title: "B", Position: 1})
	if err != nil {
		t.Fatal(err)
	}
	raceOne, raceDraft, err := repository.CreateLessonAtPosition(ctx, raceDraft.ID, raceModuleA.ID, raceDraft.Revision, authoring.NewDraftLessonInput(raceDraft.ID, raceModuleA.ID, "race-one", "One", "A lesson.", []string{"Understand"}, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	raceTwo, raceDraft, err := repository.CreateLessonAtPosition(ctx, raceDraft.ID, raceModuleB.ID, raceDraft.Revision, authoring.NewDraftLessonInput(raceDraft.ID, raceModuleB.ID, "race-two", "Two", "A lesson.", []string{"Understand"}, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, order := range [][]authoring.ModuleLessonOrder{{{ModuleID: raceModuleA.ID, LessonIDs: []authoring.LessonID{raceOne.ID}}, {ModuleID: raceModuleB.ID, LessonIDs: []authoring.LessonID{raceTwo.ID}}}, {{ModuleID: raceModuleA.ID, LessonIDs: []authoring.LessonID{raceTwo.ID}}, {ModuleID: raceModuleB.ID, LessonIDs: []authoring.LessonID{raceOne.ID}}}} {
		order := order
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := repository.ReorderLessonsForDraft(ctx, raceDraft.ID, raceDraft.Revision, order)
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
			t.Fatalf("concurrent lesson reorder = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("lesson reorder race successes=%d conflicts=%d", successes, conflicts)
	}
}

func testAuthoringLessonContentMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-lesson-content-mutation")
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
	module, draft, err := repository.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "content-module", Title: "Content", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	lesson, draft, err := repository.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, authoring.NewDraftLessonInput(draft.ID, module.ID, "content-lesson", "Content lesson", "A lesson.", []string{"Understand"}, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	authorizer := authoring.NewAuthorizationService(repository)
	contentService := authoring.NewLessonContentMutationService(repository, authorizer)
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	session, err := sessions.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	router := authTestRouter(&authHTTP{sessions: sessions, authoringLessonContent: contentService})
	cookie := &http.Cookie{Name: sessionCookieName, Value: session.Token.Value()}
	csrf := authTestCSRFToken().Value()
	path := "/api/authoring/drafts/" + string(draft.ID) + "/lessons/" + string(lesson.ID) + "/content"
	validContent := `{"schemaVersion":1,"blocks":[{"key":"divider","type":"DIVIDER","payload":{}}]}`
	body := `{"expectedLessonRevision":1,"content":` + validContent + `}`
	if response := authRequest(router, http.MethodPut, path, body, nil, csrf); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated content replacement = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPut, path, body, cookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF content replacement = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPut, path, body, cookie, "wrong"); response.Code != http.StatusForbidden {
		t.Fatalf("wrong CSRF content replacement = %d", response.Code)
	}
	untrusted := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	untrusted.Header.Set("Content-Type", "application/json")
	untrusted.Header.Set("Origin", "https://attacker.example")
	untrusted.Header.Set("X-CSRF-Token", csrf)
	untrusted.AddCookie(cookie)
	untrustedResponse := httptest.NewRecorder()
	router.ServeHTTP(untrustedResponse, untrusted)
	if untrustedResponse.Code != http.StatusForbidden {
		t.Fatalf("untrusted Origin content replacement = %d", untrustedResponse.Code)
	}
	if response := authRequest(router, http.MethodPut, "/api/authoring/drafts/not-a-uuid/lessons/"+string(lesson.ID)+"/content", body, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed draft identifier = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/not-a-uuid/content", body, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed lesson identifier = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/55555555-5555-4555-8555-555555555555/content", body, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("nonexistent lesson content replacement = %d", response.Code)
	}
	response := authRequest(router, http.MethodPut, path, body, cookie, csrf)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"divider"`) || !strings.Contains(response.Body.String(), `"revision":2`) || !strings.Contains(response.Body.String(), `"draftRevision":4`) {
		t.Fatalf("content replacement = %d: %s", response.Code, response.Body.String())
	}
	updated, err := repository.GetLesson(ctx, lesson.ID)
	if err != nil || updated.Revision != 2 || len(updated.Content.Blocks) != 1 || updated.Content.Blocks[0].Key != "divider" {
		t.Fatalf("content did not persist canonically: %v %#v", err, updated)
	}
	nonmember, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	nonmemberSession, err := sessions.CreateSession(ctx, nonmember.ID)
	if err != nil {
		t.Fatal(err)
	}
	nonmemberCookie := &http.Cookie{Name: sessionCookieName, Value: nonmemberSession.Token.Value()}
	if response = authRequest(router, http.MethodPut, path, `{"expectedLessonRevision":2,"content":`+validContent+`}`, nonmemberCookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("nonmember content replacement = %d", response.Code)
	}
	draft, _ = repository.GetDraft(ctx, draft.ID)
	if draft.Revision != 4 {
		t.Fatalf("content replacement did not increment draft revision: %d", draft.Revision)
	}
	for _, request := range []string{
		`{"expectedLessonRevision":2,"content":{"schemaVersion":2,"blocks":[]}}`,
		`{"expectedLessonRevision":2,"content":` + validContent + `,"unknown":true}`,
		`{"expectedLessonRevision":2,"content":` + validContent + `} {}`,
	} {
		if response = authRequest(router, http.MethodPut, path, request, cookie, csrf); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid content request %s = %d", request, response.Code)
		}
	}
	oversized := `{"expectedLessonRevision":2,"content":` + validContent + `,"padding":"` + strings.Repeat("x", maxAuthoringLessonContentBodyBytes) + `"}`
	if response = authRequest(router, http.MethodPut, path, oversized, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized content request = %d", response.Code)
	}
	if response = authRequest(router, http.MethodPut, path, `{"expectedLessonRevision":1,"content":`+validContent+`}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("stale content replacement = %d", response.Code)
	}
	unchanged, err := repository.GetLesson(ctx, lesson.ID)
	if err != nil || unchanged.Revision != 2 || unchanged.Content.Blocks[0].Key != "divider" {
		t.Fatalf("stale update overwrote content: %v %#v", err, unchanged)
	}

	other, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	otherModule, other, err := repository.CreateModuleAtPosition(ctx, other.ID, other.Revision, authoring.ModuleInput{DraftID: other.ID, StableKey: "other-content-module", Title: "Other", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	otherLesson, _, err := repository.CreateLessonAtPosition(ctx, other.ID, otherModule.ID, other.Revision, authoring.NewDraftLessonInput(other.ID, otherModule.ID, "other-content-lesson", "Other lesson", "A lesson.", []string{"Understand"}, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	if response = authRequest(router, http.MethodPut, "/api/authoring/drafts/"+string(draft.ID)+"/lessons/"+string(otherLesson.ID)+"/content", `{"expectedLessonRevision":1,"content":`+validContent+`}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("cross-draft content replacement = %d", response.Code)
	}
	// Revocation fixtures retain another maintainer, as production writes require.
	if _, err := addTestAuthoringMember(ctx, pool, repository, workspace.ID, "77777777-7777-4777-8777-777777777777", authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	if _, err := revokeTestAuthoringMember(ctx, pool, repository, workspace.ID, string(user.ID)); err != nil {
		t.Fatal(err)
	}
	if response = authRequest(router, http.MethodPut, path, `{"expectedLessonRevision":2,"content":`+validContent+`}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("revoked content replacement = %d", response.Code)
	}

	raceDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(user.ID))
	if err != nil {
		t.Fatal(err)
	}
	raceModule, raceDraft, err := repository.CreateModuleAtPosition(ctx, raceDraft.ID, raceDraft.Revision, authoring.ModuleInput{DraftID: raceDraft.ID, StableKey: "race-content-module", Title: "Race", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	raceLesson, _, err := repository.CreateLessonAtPosition(ctx, raceDraft.ID, raceModule.ID, raceDraft.Revision, authoring.NewDraftLessonInput(raceDraft.ID, raceModule.ID, "race-content-lesson", "Race lesson", "A lesson.", []string{"Understand"}, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	firstContent := courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{{Key: "first", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}}
	secondContent := courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{{Key: "second", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, replacement := range []courses.LessonContent{firstContent, secondContent} {
		replacement := replacement
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, _, err := repository.ReplaceLessonContentForDraft(ctx, raceDraft.ID, raceLesson.ID, raceLesson.Revision, replacement)
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
			t.Fatalf("concurrent content replacement = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("content replacement race successes=%d conflicts=%d", successes, conflicts)
	}
	finalLesson, err := repository.GetLesson(ctx, raceLesson.ID)
	if err != nil || finalLesson.Revision != 2 || len(finalLesson.Content.Blocks) != 1 || (finalLesson.Content.Blocks[0].Key != "first" && finalLesson.Content.Blocks[0].Key != "second") {
		t.Fatalf("concurrent content replacement corrupted lesson: %v %#v", err, finalLesson)
	}
}

func testAuthoringMembershipMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-membership-mutation")
	if err != nil {
		t.Fatal(err)
	}
	identityRepository := identitypostgres.New(pool)
	creator, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	authorUser, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	maintainerUser, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	revokedMaintainer, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	repository := authoringpostgres.New(pool)
	draft, workspace, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(creator.ID))
	if err != nil {
		t.Fatal(err)
	}
	authorizer := authoring.NewAuthorizationService(repository)
	memberships := authoring.NewMembershipMutationService(repository, authorizer)
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	creatorSession, err := sessions.CreateSession(ctx, creator.ID)
	if err != nil {
		t.Fatal(err)
	}
	router := authTestRouter(&authHTTP{sessions: sessions, authoringMemberships: memberships})
	cookie := &http.Cookie{Name: sessionCookieName, Value: creatorSession.Token.Value()}
	csrf := authTestCSRFToken().Value()
	base := "/api/authoring/drafts/" + string(draft.ID) + "/members"
	post := func(revision int64, user identity.UserID, role authoring.MemberRole) *httptest.ResponseRecorder {
		return authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":`+strconv.FormatInt(revision, 10)+`,"userId":"`+string(user)+`","role":"`+string(role)+`"}`, cookie, csrf)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":1,"userId":"`+string(authorUser.ID)+`","role":"AUTHOR"}`, nil, csrf); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated member add = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":1,"userId":"`+string(authorUser.ID)+`","role":"AUTHOR"}`, cookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF member add = %d", response.Code)
	}
	untrusted := httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{"expectedDraftRevision":1,"userId":"`+string(authorUser.ID)+`","role":"AUTHOR"}`))
	untrusted.Header.Set("Content-Type", "application/json")
	untrusted.Header.Set("Origin", "https://attacker.example")
	untrusted.Header.Set("X-CSRF-Token", csrf)
	untrusted.AddCookie(cookie)
	untrustedResponse := httptest.NewRecorder()
	router.ServeHTTP(untrustedResponse, untrusted)
	if untrustedResponse.Code != http.StatusForbidden {
		t.Fatalf("untrusted Origin member add = %d", untrustedResponse.Code)
	}
	for _, invalid := range []string{
		`{"expectedDraftRevision":1,"userId":"` + string(authorUser.ID) + `","role":"ADMIN"}`,
		`{"expectedDraftRevision":1,"userId":"` + string(authorUser.ID) + `","role":"AUTHOR","unexpected":true}`,
		`{"expectedDraftRevision":1,"userId":"` + string(authorUser.ID) + `","role":"AUTHOR"} {}`,
	} {
		if response := authRequest(router, http.MethodPost, base, invalid, cookie, csrf); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid member add %s = %d", invalid, response.Code)
		}
	}
	if response := authRequest(router, http.MethodPost, "/api/authoring/drafts/not-a-uuid/members", `{"expectedDraftRevision":1,"userId":"`+string(authorUser.ID)+`","role":"AUTHOR"}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed draft membership path = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPatch, base+"/not-a-uuid", `{"expectedDraftRevision":1,"role":"AUTHOR"}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed member membership path = %d", response.Code)
	}
	oversized := `{"expectedDraftRevision":1,"userId":"` + string(authorUser.ID) + `","role":"AUTHOR","padding":"` + strings.Repeat("x", maxAuthoringMembershipBodyBytes) + `"}`
	if response := authRequest(router, http.MethodPost, base, oversized, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized membership request = %d", response.Code)
	}
	if response := post(1, authorUser.ID, authoring.MemberAuthor); response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"draftRevision":2`) {
		t.Fatalf("member add = %d: %s", response.Code, response.Body.String())
	}
	if response := post(2, authorUser.ID, authoring.MemberAuthor); response.Code != http.StatusConflict {
		t.Fatalf("duplicate active member = %d", response.Code)
	}
	authorSession, err := sessions.CreateSession(ctx, authorUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorCookie := &http.Cookie{Name: sessionCookieName, Value: authorSession.Token.Value()}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":2,"userId":"`+string(maintainerUser.ID)+`","role":"MAINTAINER"}`, authorCookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("AUTHOR membership management = %d", response.Code)
	}
	if response := post(2, maintainerUser.ID, authoring.MemberMaintainer); response.Code != http.StatusOK {
		t.Fatalf("maintainer add = %d: %s", response.Code, response.Body.String())
	}
	rolePath := base + "/" + string(authorUser.ID)
	if response := authRequest(router, http.MethodPatch, rolePath, `{"expectedDraftRevision":3,"role":"MAINTAINER"}`, cookie, csrf); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"draftRevision":4`) {
		t.Fatalf("AUTHOR promotion = %d: %s", response.Code, response.Body.String())
	}
	if response := authRequest(router, http.MethodPatch, base+"/"+string(creator.ID), `{"expectedDraftRevision":4,"role":"MAINTAINER"}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("no-op role change = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPatch, base+"/"+string(maintainerUser.ID), `{"expectedDraftRevision":4,"role":"AUTHOR"}`, cookie, csrf); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"draftRevision":5`) {
		t.Fatalf("maintainer demotion with replacement = %d: %s", response.Code, response.Body.String())
	}
	if response := authRequest(router, http.MethodPatch, rolePath, `{"expectedDraftRevision":4,"role":"AUTHOR"}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("stale member role update = %d", response.Code)
	}
	if response := authRequest(router, http.MethodDelete, rolePath, `{"expectedDraftRevision":5}`, cookie, csrf); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"draftRevision":6`) {
		t.Fatalf("maintainer revoke with replacement = %d: %s", response.Code, response.Body.String())
	}
	if response := authRequest(router, http.MethodDelete, base+"/"+string(creator.ID), `{"expectedDraftRevision":6}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("sole maintainer revoke = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPatch, base+"/"+string(creator.ID), `{"expectedDraftRevision":6,"role":"AUTHOR"}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("sole maintainer demotion = %d", response.Code)
	}
	if response := authRequest(router, http.MethodDelete, base+"/"+string(maintainerUser.ID), `{"expectedDraftRevision":6}`, cookie, csrf); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"draftRevision":7`) {
		t.Fatalf("AUTHOR revoke = %d: %s", response.Code, response.Body.String())
	}
	if response := post(7, revokedMaintainer.ID, authoring.MemberMaintainer); response.Code != http.StatusOK {
		t.Fatalf("replacement maintainer add = %d", response.Code)
	}
	revokedSession, err := sessions.CreateSession(ctx, revokedMaintainer.ID)
	if err != nil {
		t.Fatal(err)
	}
	revokedCookie := &http.Cookie{Name: sessionCookieName, Value: revokedSession.Token.Value()}
	if response := authRequest(router, http.MethodDelete, base+"/"+string(revokedMaintainer.ID), `{"expectedDraftRevision":8}`, cookie, csrf); response.Code != http.StatusOK {
		t.Fatalf("revoke replacement maintainer = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":9,"userId":"99999999-9999-4999-8999-999999999999","role":"AUTHOR"}`, revokedCookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("revoked maintainer membership management = %d", response.Code)
	}
	members, err := repository.ListMembers(ctx, workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	activeMaintainers := 0
	revokedHistory := 0
	for _, member := range members {
		if member.Role == authoring.MemberMaintainer && member.RevokedAt == nil {
			activeMaintainers++
		}
		if member.RevokedAt != nil {
			revokedHistory++
		}
	}
	if activeMaintainers != 1 || revokedHistory < 3 {
		t.Fatalf("membership history lost active maintainer invariant: %#v", members)
	}
	foreignUser, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(foreignUser.ID)); err != nil {
		t.Fatal(err)
	}
	currentDraft, err := repository.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := authRequest(router, http.MethodPatch, base+"/"+string(foreignUser.ID), `{"expectedDraftRevision":`+strconv.FormatInt(currentDraft.Revision, 10)+`,"role":"AUTHOR"}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("cross-draft member role update = %d", response.Code)
	}

	raceDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(creator.ID))
	if err != nil {
		t.Fatal(err)
	}
	raceOther, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	_, raceDraft, err = repository.AddMemberForDraft(ctx, raceDraft.ID, raceDraft.Revision, string(raceOther.ID), authoring.MemberMaintainer)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, userID := range []string{string(creator.ID), string(raceOther.ID)} {
		userID := userID
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, _, err := repository.RevokeMemberForDraft(ctx, raceDraft.ID, raceDraft.Revision, userID)
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
		} else if errors.Is(err, authoring.ErrConflict) || errors.Is(err, authoring.ErrRevisionMismatch) {
			conflicts++
		} else {
			t.Fatalf("concurrent maintainer revoke = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent maintainer revoke successes=%d conflicts=%d", successes, conflicts)
	}
	raceWorkspace, err := repository.GetWorkspace(ctx, raceDraft.ID)
	if err != nil {
		t.Fatal(err)
	}
	raceMembers, err := repository.ListMembers(ctx, raceWorkspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	remaining := 0
	for _, member := range raceMembers {
		if member.Role == authoring.MemberMaintainer && member.RevokedAt == nil {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("concurrent revoke left %d active maintainers", remaining)
	}
}
