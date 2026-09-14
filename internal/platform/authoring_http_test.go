package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type authoringHTTPRepository struct {
	drafts        map[authoring.DraftID]authoring.CourseDraft
	workspaces    map[authoring.DraftID]authoring.AuthoringWorkspace
	modules       map[authoring.DraftID][]authoring.DraftModule
	lessons       map[authoring.LessonID]authoring.DraftLesson
	byDraft       map[authoring.DraftID][]authoring.DraftLesson
	prerequisites map[authoring.DraftID][]authoring.Prerequisite
	err           error
}

func (r authoringHTTPRepository) GetDraft(_ context.Context, id authoring.DraftID) (authoring.CourseDraft, error) {
	if r.err != nil {
		return authoring.CourseDraft{}, r.err
	}
	value, ok := r.drafts[id]
	if !ok {
		return authoring.CourseDraft{}, authoring.ErrNotFound
	}
	return value, nil
}
func (r authoringHTTPRepository) GetWorkspace(_ context.Context, id authoring.DraftID) (authoring.AuthoringWorkspace, error) {
	if r.err != nil {
		return authoring.AuthoringWorkspace{}, r.err
	}
	value, ok := r.workspaces[id]
	if !ok {
		return authoring.AuthoringWorkspace{}, authoring.ErrNotFound
	}
	return value, nil
}
func (r authoringHTTPRepository) ListModules(_ context.Context, id authoring.DraftID) ([]authoring.DraftModule, error) {
	return r.modules[id], r.err
}
func (r authoringHTTPRepository) GetLesson(_ context.Context, id authoring.LessonID) (authoring.DraftLesson, error) {
	if r.err != nil {
		return authoring.DraftLesson{}, r.err
	}
	value, ok := r.lessons[id]
	if !ok {
		return authoring.DraftLesson{}, authoring.ErrNotFound
	}
	return value, nil
}
func (r authoringHTTPRepository) ListLessonsForDraft(_ context.Context, id authoring.DraftID) ([]authoring.DraftLesson, error) {
	return r.byDraft[id], r.err
}
func (r authoringHTTPRepository) ListPrerequisitesForDraft(_ context.Context, id authoring.DraftID) ([]authoring.Prerequisite, error) {
	return r.prerequisites[id], r.err
}

type authoringMembershipsFake struct {
	roles map[authoring.DraftID]authoring.MemberRole
	err   error
}

func (m authoringMembershipsFake) ActiveMembershipForDraft(_ context.Context, draft authoring.DraftID, _ string) (authoring.MemberRole, bool, error) {
	if m.err != nil {
		return "", false, m.err
	}
	role, ok := m.roles[draft]
	return role, ok, nil
}

func TestAuthoringReadHTTPAuthorizationDTOsAndCachePolicy(t *testing.T) {
	draftA := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	draftB := authoring.DraftID("22222222-2222-4222-8222-222222222222")
	lessonA := authoring.LessonID("33333333-3333-4333-8333-333333333333")
	lessonB := authoring.LessonID("44444444-4444-4444-8444-444444444444")
	moduleA := authoring.ModuleID("55555555-5555-4555-8555-555555555555")
	moduleB := authoring.ModuleID("66666666-6666-4666-8666-666666666666")
	content := courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{}}
	draft := authoring.CourseDraft{ID: draftA, Metadata: authoring.DraftMetadata{CourseID: "77777777-7777-4777-8777-777777777777", IntendedVersion: courses.Version{Major: 1}, SourceLanguage: "en", Title: "Draft", Description: "Description", LearningObjectives: []string{"Understand"}, Changelog: "Initial", License: courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"}}, Status: authoring.DraftActive, Revision: 4}
	first := authoring.DraftLesson{ID: lessonA, LessonInput: authoring.LessonInput{DraftID: draftA, ModuleID: moduleA, StableKey: "first-lesson", Title: "First lesson", Description: "Description", LearningObjectives: []string{"Understand"}, Position: 0, Content: content}, Revision: 3}
	other := authoring.DraftLesson{ID: lessonB, LessonInput: authoring.LessonInput{DraftID: draftB, ModuleID: moduleB, StableKey: "other-lesson", Title: "Other lesson", Description: "Description", LearningObjectives: []string{"Apply"}, Position: 0, Content: content}, Revision: 2}
	repository := authoringHTTPRepository{
		drafts:        map[authoring.DraftID]authoring.CourseDraft{draftA: draft},
		workspaces:    map[authoring.DraftID]authoring.AuthoringWorkspace{draftA: {ID: "88888888-8888-4888-8888-888888888888", DraftID: draftA}},
		modules:       map[authoring.DraftID][]authoring.DraftModule{draftA: {{ID: moduleA, ModuleInput: authoring.ModuleInput{DraftID: draftA, StableKey: "first-module", Title: "First module", Position: 0}, Revision: 2}}},
		lessons:       map[authoring.LessonID]authoring.DraftLesson{lessonA: first, lessonB: other},
		byDraft:       map[authoring.DraftID][]authoring.DraftLesson{draftA: {first}},
		prerequisites: map[authoring.DraftID][]authoring.Prerequisite{draftA: {{LessonID: lessonA, TargetStableKey: "foundation", Position: 0}}},
	}
	readService := authoring.NewReadService(repository, authoring.NewAuthorizationService(authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftA: authoring.MemberAuthor}}))
	resolver := &authResolverFake{current: loginTestCurrent(t)}
	router := authTestRouter(&authHTTP{sessions: resolver, authoring: readService})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}

	unauthenticated := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftA), "", nil)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated draft read = %d", unauthenticated.Code)
	}
	for _, path := range []string{
		"/api/authoring/drafts/" + string(draftA),
		"/api/authoring/drafts/" + string(draftA) + "/workspace",
		"/api/authoring/drafts/" + string(draftA) + "/structure",
		"/api/authoring/drafts/" + string(draftA) + "/lessons/" + string(lessonA),
	} {
		response := authRequest(router, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s = %d cache=%q body=%s", path, response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
	}
	lessonResponse := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftA)+"/lessons/"+string(lessonA), "", cookie)
	if !strings.Contains(lessonResponse.Body.String(), `"schemaVersion":1`) || strings.Contains(lessonResponse.Body.String(), "CreatedByUserID") {
		t.Fatalf("lesson DTO did not preserve canonical content or exposed persistence fields: %s", lessonResponse.Body.String())
	}

	response := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftA)+"/structure", "", cookie)
	var structure struct {
		Modules []struct {
			StableKey string `json:"stable_key"`
			Revision  int64  `json:"revision"`
			Lessons   []struct {
				StableKey     string   `json:"stable_key"`
				Revision      int64    `json:"revision"`
				Prerequisites []string `json:"recommended_prerequisite_keys"`
			} `json:"lessons"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &structure); err != nil || len(structure.Modules) != 1 || structure.Modules[0].StableKey != "first-module" || structure.Modules[0].Revision != 2 || structure.Modules[0].Lessons[0].StableKey != "first-lesson" || structure.Modules[0].Lessons[0].Revision != 3 || structure.Modules[0].Lessons[0].Prerequisites[0] != "foundation" || strings.Contains(response.Body.String(), `"content"`) {
		t.Fatalf("structure DTO lost ordering/revisions/prerequisites or leaked content: %v %s", err, response.Body.String())
	}

	for _, path := range []string{
		"/api/authoring/drafts/" + string(draftB),
		"/api/authoring/drafts/" + string(draftA) + "/lessons/" + string(lessonB),
	} {
		response := authRequest(router, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "MAINTAINER") || strings.Contains(response.Body.String(), "AUTHOR") {
			t.Fatalf("hidden resource %s = %d: %s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/authoring/drafts/not-a-uuid",
		"/api/authoring/drafts/" + string(draftA) + "/lessons/not-a-uuid",
	} {
		if response := authRequest(router, http.MethodGet, path, "", cookie); response.Code != http.StatusBadRequest {
			t.Fatalf("malformed path %s = %d", path, response.Code)
		}
	}

	failed := authoring.NewReadService(repository, authoring.NewAuthorizationService(authoringMembershipsFake{err: errors.New("database unavailable")}))
	failedRouter := authTestRouter(&authHTTP{sessions: resolver, authoring: failed})
	if response := authRequest(failedRouter, http.MethodGet, "/api/authoring/drafts/"+string(draftA), "", cookie); response.Code != http.StatusInternalServerError {
		t.Fatalf("authorization outage = %d", response.Code)
	}
	storageFailed := authoring.NewReadService(authoringHTTPRepository{err: errors.New("storage unavailable")}, authoring.NewAuthorizationService(authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftA: authoring.MemberAuthor}}))
	storageFailedRouter := authTestRouter(&authHTTP{sessions: resolver, authoring: storageFailed})
	if response := authRequest(storageFailedRouter, http.MethodGet, "/api/authoring/drafts/"+string(draftA), "", cookie); response.Code != http.StatusInternalServerError {
		t.Fatalf("storage outage = %d", response.Code)
	}
}
