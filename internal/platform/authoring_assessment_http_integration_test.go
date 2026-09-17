//go:build integration

package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	assessmentspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringAssessmentAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	identityRepository := identitypostgres.New(pool)
	author, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	editor, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	other, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	version, err := courses.ParseVersion("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	authoringRepository := authoringpostgres.New(pool)
	draft, err := authoringRepository.CreateDraftForCreator(ctx, authoring.DraftCreationInput{
		Title: "Assessment API integration", IntendedVersion: version, SourceLanguage: "en", Description: "A Draft for Assessment API integration.", LearningObjectives: []string{"Manage Assessments"}, Changelog: "Initial Draft.",
	}, string(author.ID))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := authoringRepository.GetWorkspace(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, workspace.ID, string(editor.ID), authoring.MemberAuthor); err != nil {
		t.Fatal(err)
	}
	otherDraft, err := authoringRepository.CreateDraftForCreator(ctx, authoring.DraftCreationInput{
		Title: "Other Assessment Draft", IntendedVersion: version, SourceLanguage: "en", Description: "A separate Draft for hidden-resource coverage.", LearningObjectives: []string{"Keep Drafts isolated"}, Changelog: "Initial Draft.",
	}, string(other.ID))
	if err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identityRepository, nil, nil)
	editorSession, err := sessions.CreateSession(ctx, editor.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherSession, err := sessions.CreateSession(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	service := authoring.NewAssessmentManagementService(assessmentspostgres.New(pool), authoring.NewAuthorizationService(authoringRepository))
	router := authTestRouter(&authHTTP{sessions: sessions, authoringAssessments: service})
	path := "/api/authoring/drafts/" + string(draft.ID) + "/assessments"
	create := authoringAssessmentCreateRequest{Title: "Deterministic quiz", Questions: assessmentQuestionRequest()}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, assessmentIntegrationRequest(t, http.MethodPost, path, editorSession.Token.Value(), create))
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create response=%d body=%s", response.Code, response.Body.String())
	}
	var created AuthoringAssessmentDTO
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.AssessmentKey == "" || created.Revision != 1 || len(created.Questions) != 3 || len(created.Questions[0].CorrectOptionKeys) != 1 || created.Questions[0].CorrectOptionKeys[0] != "second" {
		t.Fatalf("Authoring detail=%#v", created)
	}
	for _, private := range []string{string(author.ID), string(draft.ID), "createdByUserID", "ownerDraftID"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("detail leaked %q: %s", private, response.Body.String())
		}
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentIntegrationRequest(t, http.MethodGet, path+"?limit=1&offset=0", editorSession.Token.Value(), nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"questionCount":3`) || strings.Contains(response.Body.String(), "correctOptionKeys") {
		t.Fatalf("list response=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentIntegrationRequest(t, http.MethodGet, path+"/"+created.AssessmentKey, editorSession.Token.Value(), nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"correctOptionKeys":["second"]`) {
		t.Fatalf("read response=%d body=%s", response.Code, response.Body.String())
	}

	update := authoringAssessmentUpdateRequest{ExpectedRevision: pointer(created.Revision), Title: "Updated deterministic quiz", Questions: assessmentQuestionRequest()}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentIntegrationRequest(t, http.MethodPut, path+"/"+created.AssessmentKey, editorSession.Token.Value(), update))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"revision":2`) {
		t.Fatalf("update response=%d body=%s", response.Code, response.Body.String())
	}

	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	requests := make([]*http.Request, 0, 2)
	for _, title := range []string{"Concurrent one", "Concurrent two"} {
		requests = append(requests, assessmentIntegrationRequest(t, http.MethodPut, path+"/"+created.AssessmentKey, editorSession.Token.Value(), authoringAssessmentUpdateRequest{ExpectedRevision: pointer(int64(2)), Title: title, Questions: assessmentQuestionRequest()}))
	}
	for _, request := range requests {
		wait.Add(1)
		go func(request *http.Request) {
			defer wait.Done()
			<-start
			result := httptest.NewRecorder()
			router.ServeHTTP(result, request)
			statuses <- result.Code
		}(request)
	}
	close(start)
	wait.Wait()
	close(statuses)
	successes, conflicts := 0, 0
	for status := range statuses {
		if status == http.StatusOK {
			successes++
		} else if status == http.StatusConflict {
			conflicts++
		} else {
			t.Fatalf("concurrent update status=%d", status)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent results success=%d conflict=%d", successes, conflicts)
	}

	foreign := httptest.NewRecorder()
	router.ServeHTTP(foreign, assessmentIntegrationRequest(t, http.MethodGet, "/api/authoring/drafts/"+string(otherDraft.ID)+"/assessments/"+created.AssessmentKey, otherSession.Token.Value(), nil))
	if foreign.Code != http.StatusNotFound || strings.Contains(foreign.Body.String(), "Deterministic") || strings.Contains(foreign.Body.String(), "Updated") {
		t.Fatalf("cross-Draft response=%d body=%s", foreign.Code, foreign.Body.String())
	}
}

func assessmentIntegrationRequest(t *testing.T, method, path, token string, value any) *http.Request {
	t.Helper()
	var body *bytes.Reader
	if value == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, body)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	if method == http.MethodPost || method == http.MethodPut {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://academy.example.com")
		request.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
	}
	return request
}
