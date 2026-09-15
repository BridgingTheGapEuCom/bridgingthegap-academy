package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	members       map[authoring.DraftID][]authoring.WorkspaceMember
	err           error
}

func (r authoringHTTPRepository) ActiveMembers(_ context.Context, id authoring.DraftID) ([]authoring.WorkspaceMember, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]authoring.WorkspaceMember(nil), r.members[id]...), nil
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
func (r authoringHTTPRepository) ReadStructure(_ context.Context, id authoring.DraftID) ([]authoring.ModuleStructure, error) {
	result := make([]authoring.ModuleStructure, 0, len(r.modules[id]))
	for _, module := range r.modules[id] {
		item := authoring.ModuleStructure{Module: module}
		for _, lesson := range r.byDraft[id] {
			if lesson.ModuleID != module.ID {
				continue
			}
			keys := []string{}
			for _, p := range r.prerequisites[id] {
				if p.LessonID == lesson.ID {
					keys = append(keys, p.TargetStableKey)
				}
			}
			item.Lessons = append(item.Lessons, authoring.LessonStructure{Lesson: lesson, RecommendedPrerequisiteKeys: keys})
		}
		result = append(result, item)
	}
	return result, r.err
}
func (r authoringHTTPRepository) ReadLesson(_ context.Context, draftID authoring.DraftID, id authoring.LessonID) (authoring.DraftLesson, []authoring.Prerequisite, error) {
	if r.err != nil {
		return authoring.DraftLesson{}, nil, r.err
	}
	lesson, found := r.lessons[id]
	if !found || lesson.DraftID != draftID {
		return authoring.DraftLesson{}, nil, authoring.ErrNotFound
	}
	return lesson, r.prerequisites[draftID], nil
}

func (r *authoringHTTPRepository) UpdateDraftMetadata(_ context.Context, id authoring.DraftID, expected int64, metadata authoring.DraftMetadata) (authoring.CourseDraft, error) {
	if r.err != nil {
		return authoring.CourseDraft{}, r.err
	}
	draft, found := r.drafts[id]
	if !found {
		return authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if draft.Revision != expected {
		return authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	draft.Metadata = metadata
	draft.Revision++
	draft.UpdatedAt = time.Now().UTC()
	r.drafts[id] = draft
	return draft, nil
}

type authoringMembershipsFake struct {
	roles map[authoring.DraftID]authoring.MemberRole
	err   error
}

type authoringModuleHTTPRepository struct {
	draft   authoring.CourseDraft
	modules map[authoring.ModuleID]authoring.DraftModule
}

func (r *authoringModuleHTTPRepository) CreateModuleAtPosition(_ context.Context, draftID authoring.DraftID, expected int64, input authoring.ModuleInput) (authoring.DraftModule, authoring.CourseDraft, error) {
	if draftID != r.draft.ID {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if expected != r.draft.Revision {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	if input.Position < 0 || input.Position > len(r.modules) {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	for id, module := range r.modules {
		if module.Position >= input.Position {
			module.Position++
			module.Revision++
			r.modules[id] = module
		}
	}
	module := authoring.DraftModule{ID: authoring.ModuleID("55555555-5555-4555-8555-555555555555"), ModuleInput: input, Revision: 1}
	r.modules[module.ID] = module
	r.draft.Revision++
	return module, r.draft, nil
}

func (r *authoringModuleHTTPRepository) GetModule(_ context.Context, id authoring.ModuleID) (authoring.DraftModule, error) {
	module, found := r.modules[id]
	if !found {
		return authoring.DraftModule{}, authoring.ErrNotFound
	}
	return module, nil
}

func (r *authoringModuleHTTPRepository) UpdateModuleMetadata(_ context.Context, draftID authoring.DraftID, id authoring.ModuleID, expected int64, patch authoring.DraftModulePatch) (authoring.DraftModule, authoring.CourseDraft, error) {
	module, found := r.modules[id]
	if !found || module.DraftID != draftID {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if module.Revision != expected {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	next, err := patch.Apply(module.ModuleInput)
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	module.ModuleInput, module.Revision = next, module.Revision+1
	r.modules[id] = module
	r.draft.Revision++
	return module, r.draft, nil
}

func (r *authoringModuleHTTPRepository) ReorderModules(_ context.Context, draftID authoring.DraftID, expected int64, order []authoring.ModuleID) (authoring.CourseDraft, error) {
	if draftID != r.draft.ID {
		return authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if expected != r.draft.Revision {
		return authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	if len(order) != len(r.modules) {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	for position, id := range order {
		module, found := r.modules[id]
		if !found {
			return authoring.CourseDraft{}, authoring.ErrInvalidStructure
		}
		module.Position = position
		r.modules[id] = module
	}
	r.draft.Revision++
	return r.draft, nil
}

func (r *authoringModuleHTTPRepository) DeleteEmptyModule(_ context.Context, draftID authoring.DraftID, id authoring.ModuleID, expectedDraft, expectedModule int64) (authoring.CourseDraft, error) {
	module, found := r.modules[id]
	if !found || module.DraftID != draftID {
		return authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if expectedDraft != r.draft.Revision || expectedModule != module.Revision {
		return authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	delete(r.modules, id)
	r.draft.Revision++
	return r.draft, nil
}

func TestAuthoringModuleMutationHTTPSecurityAndStrictBodies(t *testing.T) {
	draftID := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	moduleID := authoring.ModuleID("22222222-2222-4222-8222-222222222222")
	draft := authoring.CourseDraft{ID: draftID, Metadata: authoring.DraftMetadata{CourseID: "77777777-7777-4777-8777-777777777777", IntendedVersion: courses.Version{Major: 1}, SourceLanguage: "en", Title: "Draft", Description: "Description", LearningObjectives: []string{"Understand"}, Changelog: "Initial", License: courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"}}, Status: authoring.DraftActive, Revision: 4}
	repository := &authoringModuleHTTPRepository{draft: draft, modules: map[authoring.ModuleID]authoring.DraftModule{moduleID: {ID: moduleID, ModuleInput: authoring.ModuleInput{DraftID: draftID, StableKey: "basics", Title: "Basics", Position: 0}, Revision: 2}}}
	memberships := authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftID: authoring.MemberAuthor}}
	resolver := &authResolverFake{current: loginTestCurrent(t)}
	router := authTestRouter(&authHTTP{sessions: resolver, authoringStructureMutations: authoring.NewModuleMutationService(repository, authoring.NewAuthorizationService(memberships))})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	base := "/api/authoring/drafts/" + string(draftID) + "/modules"

	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4,"stableKey":"new-module","title":"New","position":1}`, nil, csrf); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4,"stableKey":"new-module","title":"New","position":1}`, cookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF create = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4,"stableKey":"new-module","title":"New","position":1}`, cookie, "wrong"); response.Code != http.StatusForbidden {
		t.Fatalf("wrong CSRF create = %d", response.Code)
	}
	untrusted := httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{"expectedDraftRevision":4,"stableKey":"new-module","title":"New","position":1}`))
	untrusted.Header.Set("Content-Type", "application/json")
	untrusted.Header.Set("Origin", "https://attacker.example")
	untrusted.Header.Set("X-CSRF-Token", csrf)
	untrusted.AddCookie(cookie)
	untrustedResponse := httptest.NewRecorder()
	router.ServeHTTP(untrustedResponse, untrusted)
	if untrustedResponse.Code != http.StatusForbidden {
		t.Fatalf("untrusted Origin create = %d", untrustedResponse.Code)
	}
	response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4,"stableKey":"new-module","title":"New","position":1}`, cookie, csrf)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"draftRevision":5`) {
		t.Fatalf("create = %d: %s", response.Code, response.Body.String())
	}
	patchPath := base + "/" + string(moduleID)
	response = authRequest(router, http.MethodPatch, patchPath, `{"expectedModuleRevision":2,"title":"Renamed"}`, cookie, csrf)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"stable_key":"basics"`) || !strings.Contains(response.Body.String(), `"draftRevision":6`) {
		t.Fatalf("update = %d: %s", response.Code, response.Body.String())
	}
	for _, body := range []string{
		`{"expectedModuleRevision":3}`,
		`{"expectedModuleRevision":3,"stableKey":"renamed"}`,
		`{"expectedModuleRevision":3,"title":"Renamed"} {}`,
	} {
		if response = authRequest(router, http.MethodPatch, patchPath, body, cookie, csrf); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid module PATCH %s = %d", body, response.Code)
		}
	}
	response = authRequest(router, http.MethodPut, base+"/order", `{"expectedDraftRevision":6,"moduleIds":["`+string(moduleID)+`","55555555-5555-4555-8555-555555555555"]}`, cookie, csrf)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"draftRevision":7`) {
		t.Fatalf("reorder = %d: %s", response.Code, response.Body.String())
	}
	if response = authRequest(router, http.MethodPut, base+"/order", `{"expectedDraftRevision":7,"moduleIds":["`+string(moduleID)+`","`+string(moduleID)+`"]}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate reorder = %d", response.Code)
	}
	if response = authRequest(router, http.MethodDelete, patchPath, `{"expectedDraftRevision":6,"expectedModuleRevision":3}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("stale delete = %d", response.Code)
	}
	oversized := `{"expectedDraftRevision":7,"stableKey":"too-large","title":"` + strings.Repeat("x", maxAuthoringModuleBodyBytes) + `","position":2}`
	if response = authRequest(router, http.MethodPost, base, oversized, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized module create = %d", response.Code)
	}
	delete(memberships.roles, draftID)
	if response = authRequest(router, http.MethodPatch, patchPath, `{"expectedModuleRevision":3,"title":"Hidden"}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("nonmember update = %d", response.Code)
	}
}

func TestAuthoringDraftMetadataPATCHSecurityAndConcurrency(t *testing.T) {
	draftA := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	draftB := authoring.DraftID("22222222-2222-4222-8222-222222222222")
	draft := authoring.CourseDraft{ID: draftA, Metadata: authoring.DraftMetadata{CourseID: "77777777-7777-4777-8777-777777777777", IntendedVersion: courses.Version{Major: 1}, SourceLanguage: "en", Title: "Original", Description: "Description", LearningObjectives: []string{"Understand"}, Changelog: "Initial", License: courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"}}, Status: authoring.DraftActive, Revision: 4, UpdatedAt: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)}
	repository := &authoringHTTPRepository{drafts: map[authoring.DraftID]authoring.CourseDraft{draftA: draft}}
	memberships := authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftA: authoring.MemberAuthor}}
	authorizer := authoring.NewAuthorizationService(memberships)
	readService := authoring.NewReadService(repository, authorizer)
	mutationService := authoring.NewDraftMutationService(repository, authorizer)
	resolver := &authResolverFake{current: loginTestCurrent(t)}
	router := authTestRouter(&authHTTP{sessions: resolver, authoring: readService, authoringMutations: mutationService})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	path := "/api/authoring/drafts/" + string(draftA)

	if response := authRequest(router, http.MethodPatch, path, `{"expectedRevision":4,"title":"Updated"}`, nil, csrf); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated PATCH = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPatch, path, `{"expectedRevision":4,"title":"Updated"}`, cookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF PATCH = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPatch, path, `{"expectedRevision":4,"title":"Updated"}`, cookie, "wrong"); response.Code != http.StatusForbidden {
		t.Fatalf("wrong CSRF PATCH = %d", response.Code)
	}
	untrusted := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"expectedRevision":4,"title":"Updated"}`))
	untrusted.Header.Set("Content-Type", "application/json")
	untrusted.Header.Set("Origin", "https://attacker.example")
	untrusted.Header.Set("X-CSRF-Token", csrf)
	untrusted.AddCookie(cookie)
	untrustedResponse := httptest.NewRecorder()
	router.ServeHTTP(untrustedResponse, untrusted)
	if untrustedResponse.Code != http.StatusForbidden {
		t.Fatalf("untrusted Origin PATCH = %d", untrustedResponse.Code)
	}

	response := authRequest(router, http.MethodPatch, path, `{"expectedRevision":4,"title":"Updated","objectives":["First","Second"]}`, cookie, csrf)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"revision":5`) || !strings.Contains(response.Body.String(), `"title":"Updated"`) {
		t.Fatalf("author PATCH failed: status=%d body=%s", response.Code, response.Body.String())
	}
	if got := repository.drafts[draftA]; got.Metadata.Title != "Updated" || got.Metadata.Description != "Description" || got.Revision != 5 || !got.UpdatedAt.After(draft.UpdatedAt) {
		t.Fatalf("PATCH did not preserve omitted fields or committed revision: %#v", got)
	}

	// MAINTAINER has the same draft-edit capability without handler role logic.
	memberships.roles[draftA] = authoring.MemberMaintainer
	response = authRequest(router, http.MethodPatch, path, `{"expectedRevision":5,"description":"Maintainer update"}`, cookie, csrf)
	if response.Code != http.StatusOK || repository.drafts[draftA].Metadata.Description != "Maintainer update" || repository.drafts[draftA].Revision != 6 {
		t.Fatalf("maintainer PATCH failed: status=%d body=%s", response.Code, response.Body.String())
	}

	for _, body := range []string{
		`{`,
		`{"expectedRevision":6,"titel":"typo"}`,
		`{"expectedRevision":6,"courseId":"immutable"}`,
		`{"expectedRevision":6}`,
		`{"expectedRevision":6,"intendedVersion":"1.bad.0"}`,
		`{"expectedRevision":6,"sourceLanguage":"not a language"}`,
		`{"expectedRevision":6,"title":""}`,
		`{"expectedRevision":6,"objectives":null}`,
		`{"expectedRevision":6,"license":{"kind":"STANDARD","display_name":"Standard"}}`,
		`{"expectedRevision":6,"license":{"kind":"ALL_RIGHTS_RESERVED","display_name":"All Rights Reserved","unknown":"field"}}`,
	} {
		response = authRequest(router, http.MethodPatch, path, body, cookie, csrf)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid PATCH %s = %d: %s", body, response.Code, response.Body.String())
		}
	}
	if response = authRequest(router, http.MethodPatch, path, `{"expectedRevision":5,"title":"Stale"}`, cookie, csrf); response.Code != http.StatusConflict || repository.drafts[draftA].Metadata.Title != "Updated" || repository.drafts[draftA].Revision != 6 {
		t.Fatalf("stale PATCH overwrote current draft: status=%d draft=%#v", response.Code, repository.drafts[draftA])
	}
	if response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/"+string(draftB), `{"expectedRevision":1,"title":"Hidden"}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("nonmember PATCH = %d", response.Code)
	}
	delete(memberships.roles, draftA)
	if response = authRequest(router, http.MethodPatch, path, `{"expectedRevision":6,"title":"Revoked"}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("revoked PATCH = %d", response.Code)
	}
	if response = authRequest(router, http.MethodPatch, "/api/authoring/drafts/not-a-uuid", `{"expectedRevision":6,"title":"Bad"}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed identifier PATCH = %d", response.Code)
	}
	memberships.roles[draftA] = authoring.MemberMaintainer
	oversized := `{"expectedRevision":6,"changelog":"` + strings.Repeat("x", maxAuthoringDraftMetadataBodyBytes) + `"}`
	response = authRequest(router, http.MethodPatch, path, oversized, cookie, csrf)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized PATCH = %d", response.Code)
	}

	failedAuthorizer := authoring.NewAuthorizationService(authoringMembershipsFake{err: errors.New("database unavailable")})
	failedRouter := authTestRouter(&authHTTP{sessions: resolver, authoring: readService, authoringMutations: authoring.NewDraftMutationService(repository, failedAuthorizer)})
	if response = authRequest(failedRouter, http.MethodPatch, path, `{"expectedRevision":6,"title":"Unavailable"}`, cookie, csrf); response.Code != http.StatusInternalServerError {
		t.Fatalf("authorization outage PATCH = %d", response.Code)
	}
	storageFailed := &authoringHTTPRepository{drafts: map[authoring.DraftID]authoring.CourseDraft{draftA: repository.drafts[draftA]}, err: errors.New("storage unavailable")}
	storageFailedRouter := authTestRouter(&authHTTP{sessions: resolver, authoring: authoring.NewReadService(storageFailed, authorizer), authoringMutations: authoring.NewDraftMutationService(storageFailed, authorizer)})
	if response = authRequest(storageFailedRouter, http.MethodPatch, path, `{"expectedRevision":6,"title":"Unavailable"}`, cookie, csrf); response.Code != http.StatusInternalServerError {
		t.Fatalf("storage outage PATCH = %d", response.Code)
	}
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
		members: map[authoring.DraftID][]authoring.WorkspaceMember{draftA: {
			{UserID: "00000000-0000-4000-8000-000000000001", Role: authoring.MemberAuthor},
			{UserID: "00000000-0000-4000-8000-000000000002", Role: authoring.MemberMaintainer},
		}},
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
		"/api/authoring/drafts/" + string(draftA) + "/members",
		"/api/authoring/drafts/" + string(draftA) + "/structure",
		"/api/authoring/drafts/" + string(draftA) + "/lessons/" + string(lessonA),
	} {
		response := authRequest(router, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s = %d cache=%q body=%s", path, response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
	}
	membersResponse := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftA)+"/members", "", cookie)
	if !strings.Contains(membersResponse.Body.String(), `"userId":"00000000-0000-4000-8000-000000000001"`) || !strings.Contains(membersResponse.Body.String(), `"role":"AUTHOR"`) || strings.Contains(membersResponse.Body.String(), `"createdAt"`) || strings.Contains(membersResponse.Body.String(), `"revokedAt"`) || strings.Contains(membersResponse.Body.String(), `"id"`) {
		t.Fatalf("active membership DTO leaked persistence fields or lost opaque roles: %s", membersResponse.Body.String())
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
		"/api/authoring/drafts/" + string(draftB) + "/members",
		"/api/authoring/drafts/" + string(draftA) + "/lessons/" + string(lessonB),
	} {
		response := authRequest(router, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "MAINTAINER") || strings.Contains(response.Body.String(), "AUTHOR") {
			t.Fatalf("hidden resource %s = %d: %s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{
		"/api/authoring/drafts/not-a-uuid",
		"/api/authoring/drafts/not-a-uuid/members",
		"/api/authoring/drafts/" + string(draftA) + "/lessons/not-a-uuid",
	} {
		if response := authRequest(router, http.MethodGet, path, "", cookie); response.Code != http.StatusBadRequest {
			t.Fatalf("malformed path %s = %d", path, response.Code)
		}
	}

	failed := authoring.NewReadService(repository, authoring.NewAuthorizationService(authoringMembershipsFake{err: errors.New("database unavailable")}))
	failedRouter := authTestRouter(&authHTTP{sessions: resolver, authoring: failed})
	for _, path := range []string{
		"/api/authoring/drafts/" + string(draftA),
		"/api/authoring/drafts/" + string(draftA) + "/members",
	} {
		if response := authRequest(failedRouter, http.MethodGet, path, "", cookie); response.Code != http.StatusInternalServerError {
			t.Fatalf("authorization outage for %s = %d", path, response.Code)
		}
	}
	storageFailed := authoring.NewReadService(authoringHTTPRepository{err: errors.New("storage unavailable")}, authoring.NewAuthorizationService(authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftA: authoring.MemberAuthor}}))
	storageFailedRouter := authTestRouter(&authHTTP{sessions: resolver, authoring: storageFailed})
	for _, path := range []string{
		"/api/authoring/drafts/" + string(draftA),
		"/api/authoring/drafts/" + string(draftA) + "/members",
	} {
		if response := authRequest(storageFailedRouter, http.MethodGet, path, "", cookie); response.Code != http.StatusInternalServerError {
			t.Fatalf("storage outage for %s = %d", path, response.Code)
		}
	}
}
