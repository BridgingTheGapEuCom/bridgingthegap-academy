package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

const assessmentHTTPDraftID = authoring.DraftID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
const assessmentHTTPUserID = identity.UserID("dddddddd-dddd-4ddd-8ddd-dddddddddddd")

type assessmentHTTPRepository struct {
	assessment assessments.Assessment
	create     assessments.AssessmentInput
	update     assessments.AssessmentUpdate
	expected   int64
	list       []assessments.AssessmentSummary
	total      int
	createErr  error
	getErr     error
	updateErr  error
	listErr    error
}

func (r *assessmentHTTPRepository) CreateAssessment(_ context.Context, input assessments.AssessmentInput) (assessments.Assessment, error) {
	r.create = input
	if r.createErr != nil {
		return assessments.Assessment{}, r.createErr
	}
	if err := input.Validate(); err != nil {
		return assessments.Assessment{}, err
	}
	r.assessment = assessments.Assessment{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", OwnerDraftID: input.OwnerDraftID, CreatedByUserID: input.CreatedByUserID, Title: input.Title, Questions: input.Questions, Revision: 1, CreatedAt: assessmentHTTPTime, UpdatedAt: assessmentHTTPTime}
	return r.assessment, nil
}
func (r *assessmentHTTPRepository) GetAssessment(context.Context, assessments.AssessmentID) (assessments.Assessment, error) {
	if r.getErr != nil {
		return assessments.Assessment{}, r.getErr
	}
	return r.assessment, nil
}
func (r *assessmentHTTPRepository) UpdateAssessment(_ context.Context, _ assessments.AssessmentID, expected int64, update assessments.AssessmentUpdate) (assessments.Assessment, error) {
	r.expected, r.update = expected, update
	if r.updateErr != nil {
		return assessments.Assessment{}, r.updateErr
	}
	if expected != r.assessment.Revision {
		return assessments.Assessment{}, assessments.ErrRevisionMismatch
	}
	if err := update.Validate(); err != nil {
		return assessments.Assessment{}, err
	}
	r.assessment.Title, r.assessment.Questions, r.assessment.Revision = update.Title, update.Questions, r.assessment.Revision+1
	r.assessment.UpdatedAt = assessmentHTTPTime.Add(time.Minute)
	return r.assessment, nil
}
func (r *assessmentHTTPRepository) ListAssessmentSummariesForDraft(context.Context, string, int, int) ([]assessments.AssessmentSummary, int, error) {
	return r.list, r.total, r.listErr
}

var assessmentHTTPTime = time.Date(2026, 9, 17, 14, 0, 0, 0, time.UTC)

func assessmentHTTPRouter(t *testing.T, roles map[authoring.DraftID]authoring.MemberRole, repository *assessmentHTTPRepository) http.Handler {
	t.Helper()
	service := authoring.NewAssessmentManagementService(repository, authoring.NewAuthorizationService(&authoringMembershipsFake{roles: roles}))
	return authTestRouter(&authHTTP{sessions: &authResolverFake{current: assessmentHTTPCurrent(t)}, authoringAssessments: service})
}

type assessmentHTTPSessionRepository struct{ loginSessionRepositoryFake }

func (r *assessmentHTTPSessionRepository) GetUser(_ context.Context, id identity.UserID) (identity.User, error) {
	return identity.User{ID: id, Status: identity.UserActive}, nil
}

func assessmentHTTPCurrent(t *testing.T) identity.ResolvedSession {
	t.Helper()
	repository := &assessmentHTTPSessionRepository{loginSessionRepositoryFake: loginSessionRepositoryFake{at: assessmentHTTPTime}}
	service := identity.NewSessionService(repository, nil, func() time.Time { return repository.at })
	created, err := service.CreateSession(context.Background(), assessmentHTTPUserID)
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.ResolveSession(context.Background(), created.Token.Value())
	if err != nil {
		t.Fatal(err)
	}
	return current
}

func assessmentQuestionRequest() []authoringAssessmentQuestionDTO {
	return []authoringAssessmentQuestionDTO{
		{
			StableKey: "single", Type: assessments.QuestionSingleChoice, Prompt: "Choose one.", Position: 0,
			Options:           []authoringAssessmentOptionDTO{{StableKey: "first", Text: "First", Position: 0}, {StableKey: "second", Text: "Second", Position: 1}},
			CorrectOptionKeys: []string{"second"}, LeftItems: []authoringAssessmentMatchingDTO{}, RightItems: []authoringAssessmentMatchingDTO{}, CorrectPairs: []authoringAssessmentPairDTO{},
		},
		{
			StableKey: "multiple", Type: assessments.QuestionMultipleChoice, Prompt: "Choose all.", Position: 1,
			Options:           []authoringAssessmentOptionDTO{{StableKey: "first", Text: "First", Position: 0}, {StableKey: "second", Text: "Second", Position: 1}},
			CorrectOptionKeys: []string{"first", "second"}, LeftItems: []authoringAssessmentMatchingDTO{}, RightItems: []authoringAssessmentMatchingDTO{}, CorrectPairs: []authoringAssessmentPairDTO{},
		},
		{
			StableKey: "matching", Type: assessments.QuestionMatching, Prompt: "Match.", Position: 2,
			Options: []authoringAssessmentOptionDTO{}, CorrectOptionKeys: []string{},
			LeftItems:    []authoringAssessmentMatchingDTO{{StableKey: "left-a", Text: "A", Position: 0}, {StableKey: "left-b", Text: "B", Position: 1}},
			RightItems:   []authoringAssessmentMatchingDTO{{StableKey: "right-one", Text: "One", Position: 0}, {StableKey: "right-two", Text: "Two", Position: 1}},
			CorrectPairs: []authoringAssessmentPairDTO{{LeftKey: "left-a", RightKey: "right-two"}, {LeftKey: "left-b", RightKey: "right-one"}},
		},
	}
}

func assessmentRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://academy.example.com")
	request.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()})
	return request
}

func TestAuthoringAssessmentHTTPCreateReadListUpdateAndPrivacy(t *testing.T) {
	repository := &assessmentHTTPRepository{}
	router := assessmentHTTPRouter(t, map[authoring.DraftID]authoring.MemberRole{assessmentHTTPDraftID: authoring.MemberAuthor}, repository)
	path := "/api/authoring/drafts/" + string(assessmentHTTPDraftID) + "/assessments"
	create := authoringAssessmentCreateRequest{Title: "Knowledge check", Questions: assessmentQuestionRequest()}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodPost, path, create))
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Location") != path+"/"+string(repository.assessment.ID) {
		t.Fatalf("create response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	for _, expected := range []string{`"assessmentKey":"` + string(repository.assessment.ID) + `"`, `"correctOptionKeys":["second"]`, `"revision":1`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("Authoring detail omitted %s: %s", expected, response.Body.String())
		}
	}
	for _, private := range []string{string(assessmentHTTPUserID), string(assessmentHTTPDraftID), "createdByUserID", "ownerDraftID"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("detail leaked private provenance %q: %s", private, response.Body.String())
		}
	}
	if repository.create.OwnerDraftID != string(assessmentHTTPDraftID) || repository.create.CreatedByUserID != string(assessmentHTTPUserID) {
		t.Fatalf("server scope not used: %#v", repository.create)
	}

	repository.list = []assessments.AssessmentSummary{{ID: repository.assessment.ID, Title: repository.assessment.Title, QuestionCount: 3, Revision: 1, UpdatedAt: assessmentHTTPTime}}
	repository.total = 1
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodGet, path+"?limit=1&offset=0", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"questionCount":3`) || strings.Contains(response.Body.String(), "correctOptionKeys") || strings.Contains(response.Body.String(), string(assessmentHTTPUserID)) {
		t.Fatalf("summary response=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodGet, path+"/"+string(repository.assessment.ID), nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"correctOptionKeys":["second"]`) {
		t.Fatalf("read response=%d body=%s", response.Code, response.Body.String())
	}

	reordered := assessmentQuestionRequest()
	reordered[0], reordered[2] = reordered[2], reordered[0]
	reordered[0].Position, reordered[2].Position = 0, 2
	reordered[2].CorrectOptionKeys = []string{"first"}
	update := authoringAssessmentUpdateRequest{ExpectedRevision: pointer(int64(1)), Title: "Updated", Questions: reordered}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodPut, path+"/"+string(repository.assessment.ID), update))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"revision":2`) || !strings.Contains(response.Body.String(), `"stableKey":"matching"`) || !strings.Contains(response.Body.String(), `"correctOptionKeys":["first"]`) || repository.expected != 1 {
		t.Fatalf("update response=%d expected=%d body=%s", response.Code, repository.expected, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodPut, path+"/"+string(repository.assessment.ID), update))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"assessment_revision_conflict"`) {
		t.Fatalf("stale update response=%d body=%s", response.Code, response.Body.String())
	}
	invalid := authoringAssessmentUpdateRequest{ExpectedRevision: pointer(int64(2)), Title: "", Questions: []authoringAssessmentQuestionDTO{}}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodPut, path+"/"+string(repository.assessment.ID), invalid))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_assessment"`) || repository.assessment.Revision != 2 {
		t.Fatalf("invalid update response=%d revision=%d body=%s", response.Code, repository.assessment.Revision, response.Body.String())
	}
}

func TestAuthoringAssessmentHTTPAllowsEmptyMutableAssessment(t *testing.T) {
	repository := &assessmentHTTPRepository{}
	router := assessmentHTTPRouter(t, map[authoring.DraftID]authoring.MemberRole{assessmentHTTPDraftID: authoring.MemberMaintainer}, repository)
	path := "/api/authoring/drafts/" + string(assessmentHTTPDraftID) + "/assessments"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodPost, path, authoringAssessmentCreateRequest{Title: "Draft later", Questions: []authoringAssessmentQuestionDTO{}}))
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"questions":[]`) {
		t.Fatalf("empty create response=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodPut, path+"/"+string(repository.assessment.ID), authoringAssessmentUpdateRequest{ExpectedRevision: pointer(int64(1)), Title: "Still empty", Questions: []authoringAssessmentQuestionDTO{}}))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"revision":2`) {
		t.Fatalf("empty update response=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAuthoringAssessmentHTTPSecurityAndInvalidRequests(t *testing.T) {
	path := "/api/authoring/drafts/" + string(assessmentHTTPDraftID) + "/assessments"
	for _, test := range []struct {
		name   string
		roles  map[authoring.DraftID]authoring.MemberRole
		change func(*http.Request)
		want   int
	}{
		{name: "unauthenticated", roles: map[authoring.DraftID]authoring.MemberRole{assessmentHTTPDraftID: authoring.MemberAuthor}, change: removeCookie, want: http.StatusUnauthorized},
		{name: "missing csrf", roles: map[authoring.DraftID]authoring.MemberRole{assessmentHTTPDraftID: authoring.MemberAuthor}, change: func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, want: http.StatusForbidden},
		{name: "untrusted origin", roles: map[authoring.DraftID]authoring.MemberRole{assessmentHTTPDraftID: authoring.MemberAuthor}, change: func(r *http.Request) { r.Header.Set("Origin", "https://attacker.example") }, want: http.StatusForbidden},
		{name: "other Draft member", roles: map[authoring.DraftID]authoring.MemberRole{"cccccccc-cccc-4ccc-8ccc-cccccccccccc": authoring.MemberMaintainer}, want: http.StatusNotFound},
		{name: "administrator has no bypass", roles: map[authoring.DraftID]authoring.MemberRole{}, want: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &assessmentHTTPRepository{}
			request := assessmentRequest(t, http.MethodPost, path, authoringAssessmentCreateRequest{Title: "Empty is valid", Questions: []authoringAssessmentQuestionDTO{}})
			if test.change != nil {
				test.change(request)
			}
			response := httptest.NewRecorder()
			assessmentHTTPRouter(t, test.roles, repository).ServeHTTP(response, request)
			if response.Code != test.want || (response.Code == http.StatusNotFound && (strings.Contains(response.Body.String(), "membership") || strings.Contains(response.Body.String(), "capability"))) {
				t.Fatalf("response=%d body=%s", response.Code, response.Body.String())
			}
		})
	}

	repository := &assessmentHTTPRepository{}
	router := assessmentHTTPRouter(t, map[authoring.DraftID]authoring.MemberRole{assessmentHTTPDraftID: authoring.MemberMaintainer}, repository)
	for _, body := range []string{
		`{"title":"Invalid","questions":[],"creatorId":"attacker"}`,
		`{"title":"Invalid","questions":[],"ownerDraftId":"attacker"}`,
		`{"title":"Invalid","questions":[{"stableKey":"essay","type":"ESSAY","prompt":"No","position":0,"options":[],"correctOptionKeys":[],"leftItems":[],"rightItems":[],"correctPairs":[]}]}`,
	} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://academy.example.com")
		request.Header.Set("X-CSRF-Token", authTestCSRFToken().Value())
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_assessment"`) {
			t.Fatalf("invalid response=%d body=%s", response.Code, response.Body.String())
		}
	}

	foreign := assessments.Assessment{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", OwnerDraftID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", CreatedByUserID: string(assessmentHTTPUserID), Title: "Foreign", Revision: 1, CreatedAt: assessmentHTTPTime, UpdatedAt: assessmentHTTPTime}
	repository.assessment = foreign
	response := httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodGet, path+"/"+string(foreign.ID), nil))
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "Foreign") {
		t.Fatalf("foreign read response=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodPut, path+"/"+string(foreign.ID), authoringAssessmentUpdateRequest{ExpectedRevision: pointer(int64(1)), Title: "Steal", Questions: []authoringAssessmentQuestionDTO{}}))
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "Foreign") || strings.Contains(response.Body.String(), "Steal") {
		t.Fatalf("foreign update response=%d body=%s", response.Code, response.Body.String())
	}

	repository.getErr = errors.New("SQLSTATE 08006 secret")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, assessmentRequest(t, http.MethodGet, path+"/"+string(foreign.ID), nil))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "SQLSTATE") {
		t.Fatalf("operational response=%d body=%s", response.Code, response.Body.String())
	}
}

func pointer(value int64) *int64 { return &value }
