package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/google/uuid"
)

func TestAuthoringJSONRejectsAmbiguousAndInvalidInputs(t *testing.T) {
	decode := func(body string, kind string) error {
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		switch kind {
		case "draft":
			_, _, err := decodeAuthoringDraftUpdate(w, r)
			return err
		case "modules":
			_, _, err := decodeAuthoringModuleReorder(w, r)
			return err
		case "lessons":
			_, _, err := decodeAuthoringLessonReorder(w, r)
			return err
		case "module-create":
			_, _, err := decodeAuthoringModuleCreate(w, r, authoring.DraftID("11111111-1111-4111-8111-111111111111"))
			return err
		case "content":
			_, _, err := decodeAuthoringLessonContent(w, r)
			return err
		case "draft-create":
			_, err := decodeAuthoringDraftCreate(w, r)
			return err
		default:
			_, _, err := decodeAuthoringMemberRole(w, r)
			return err
		}
	}
	for _, test := range []struct{ name, kind, body string }{
		{"missing module array", "modules", `{"expectedDraftRevision":1}`},
		{"null module array", "modules", `{"expectedDraftRevision":1,"moduleIds":null}`},
		{"missing layout", "lessons", `{"expectedDraftRevision":1}`},
		{"null layout", "lessons", `{"expectedDraftRevision":1,"modules":null}`},
		{"missing lesson array", "lessons", `{"expectedDraftRevision":1,"modules":[{"moduleId":"11111111-1111-4111-8111-111111111111"}]}`},
		{"null lesson array", "lessons", `{"expectedDraftRevision":1,"modules":[{"moduleId":"11111111-1111-4111-8111-111111111111","lessonIds":null}]}`},
		{"null optional description", "module-create", `{"expectedDraftRevision":1,"stableKey":"intro","title":"Intro","description":null,"position":0}`},
		{"draft creation unknown creator", "draft-create", `{"title":"New Draft","intendedVersion":"0.1.0","sourceLanguage":"en","description":"Description","objectives":["Explain"],"changelog":"Initial","creatorId":"forged"}`},
		{"draft creation missing metadata", "draft-create", `{"title":"New Draft","intendedVersion":"0.1.0","sourceLanguage":"en"}`},
		{"duplicate revision", "members", `{"expectedDraftRevision":1,"expectedDraftRevision":2,"role":"AUTHOR"}`},
		{"wrong case", "members", `{"ExpectedDraftRevision":1,"role":"AUTHOR"}`},
		{"null nested license value", "draft", `{"expectedRevision":1,"license":{"kind":"ALL_RIGHTS_RESERVED","display_name":"All rights reserved","url":null}}`},
		{"null role", "members", `{"expectedDraftRevision":1,"role":null}`},
		{"unknown", "members", `{"expectedDraftRevision":1,"role":"AUTHOR","actorId":"forged"}`},
		{"trailing", "members", `{"expectedDraftRevision":1,"role":"AUTHOR"} {}`},
		{"malformed", "members", `{"expectedDraftRevision":`},
		{"duplicate canonical payload", "content", `{"expectedLessonRevision":1,"content":{"schemaVersion":1,"blocks":[{"key":"divider","type":"DIVIDER","payload":{},"payload":{}}]}}`},
		{"oversized", "members", `{"expectedDraftRevision":1,"role":"AUTHOR"}` + strings.Repeat(" ", maxAuthoringMembershipBodyBytes)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := decode(test.body, test.kind); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
	for _, test := range []struct{ kind, body string }{
		{"modules", `{"expectedDraftRevision":1,"moduleIds":[]}`},
		{"lessons", `{"expectedDraftRevision":1,"modules":[]}`},
		{"module-create", `{"expectedDraftRevision":1,"stableKey":"intro","title":"Intro","position":0}`},
		{"content", `{"expectedLessonRevision":1,"content":{"schemaVersion":1,"blocks":[]}}`},
		{"draft-create", `{"title":"New Draft","intendedVersion":"0.1.0","sourceLanguage":"en","description":"Description","objectives":["Explain"],"changelog":"Initial"}`},
	} {
		if err := decode(test.body, test.kind); err != nil {
			t.Fatalf("valid %s request rejected: %v", test.kind, err)
		}
	}
}

func TestCompleteBoundedLessonLayoutFitsRequestLimit(t *testing.T) {
	// Exercise the maximum supported structure, which exceeded the old 256 KiB limit.
	modules := make([]authoringLessonOrderModuleRequest, authoring.MaxModulesPerDraft)
	for i := range modules {
		ids := []string{}
		for j := 0; j < authoring.MaxLessonsPerDraft/authoring.MaxModulesPerDraft; j++ {
			ids = append(ids, uuid.NewString())
		}
		modules[i] = authoringLessonOrderModuleRequest{ModuleID: uuid.NewString(), LessonIDs: &ids}
	}
	rev := int64(1)
	data, err := json.Marshal(authoringLessonReorderRequest{ExpectedDraftRevision: &rev, Modules: &modules})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(string(data)))
	r.Header.Set("Content-Type", "application/json")
	_, order, err := decodeAuthoringLessonReorder(httptest.NewRecorder(), r)
	if err != nil || len(order) != authoring.MaxModulesPerDraft {
		t.Fatalf("complete bounded layout rejected (%d bytes): %v", len(data), err)
	}
}

func TestEveryAuthoringMutationInheritsBrowserWriteProtection(t *testing.T) {
	authorizer := authoring.NewAuthorizationService(authoringMembershipsFake{})
	router := authTestRouter(&authHTTP{
		sessions:                    &authResolverFake{current: loginTestCurrent(t)},
		authoringMutations:          authoring.NewDraftMutationService(nil, authorizer),
		authoringStructureMutations: authoring.NewModuleMutationService(nil, authorizer),
		authoringLessonMutations:    authoring.NewLessonMutationService(nil, authorizer),
		authoringLessonContent:      authoring.NewLessonContentMutationService(nil, authorizer),
		authoringMemberships:        authoring.NewMembershipMutationService(nil, authorizer),
		authoringCreation:           authoring.NewDraftCreationService(nil),
	})
	base := "/api/authoring/drafts/11111111-1111-4111-8111-111111111111"
	child := "22222222-2222-4222-8222-222222222222"
	routes := []struct{ method, path string }{
		{"PATCH", base}, {"POST", base + "/modules"}, {"PATCH", base + "/modules/" + child}, {"PUT", base + "/modules/order"}, {"DELETE", base + "/modules/" + child},
		{"POST", base + "/modules/" + child + "/lessons"}, {"PATCH", base + "/lessons/" + child}, {"PUT", base + "/lessons/order"}, {"PUT", base + "/lessons/" + child + "/prerequisites"}, {"DELETE", base + "/lessons/" + child}, {"PUT", base + "/lessons/" + child + "/content"},
		{"POST", "/api/authoring/drafts"}, {"POST", base + "/members"}, {"PATCH", base + "/members/" + child}, {"DELETE", base + "/members/" + child},
	}
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			for _, test := range []struct {
				name, origin, csrf string
				authenticated      bool
				status             int
			}{
				{"no session", "https://academy.example.com", authTestCSRFToken().Value(), false, 401},
				{"missing Origin", "", authTestCSRFToken().Value(), true, 403},
				{"untrusted Origin", "https://attacker.example", authTestCSRFToken().Value(), true, 403},
				{"missing CSRF", "https://academy.example.com", "", true, 403},
				{"invalid CSRF", "https://academy.example.com", "wrong", true, 403},
			} {
				t.Run(test.name, func(t *testing.T) {
					r := httptest.NewRequest(route.method, route.path, strings.NewReader("not JSON"))
					if test.origin != "" {
						r.Header.Set("Origin", test.origin)
					}
					if test.csrf != "" {
						r.Header.Set("X-CSRF-Token", test.csrf)
					}
					if test.authenticated {
						r.AddCookie(cookie)
					}
					w := httptest.NewRecorder()
					router.ServeHTTP(w, r)
					if w.Code != test.status || w.Header().Get("Cache-Control") != "no-store" {
						t.Fatalf("status=%d cache=%q", w.Code, w.Header().Get("Cache-Control"))
					}
				})
			}
		})
	}
}
