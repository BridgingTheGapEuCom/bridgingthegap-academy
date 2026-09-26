//go:build integration

package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	assessmentspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testLearnerAssessmentAttemptAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(courseRepository)
	course, err := courseRepository.CreateCourse(ctx, "learner-attempt-api")
	if err != nil {
		t.Fatal(err)
	}
	published, err := store.Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "a1000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	identities := identitypostgres.New(pool)
	learner, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	other, err := identities.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	sessions := identity.NewSessionService(identities, nil, nil)
	learnerSession, err := sessions.CreateSession(ctx, learner.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherSession, err := sessions.CreateSession(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	service := assessments.NewLearnerAttemptService(assessmentspostgres.New(pool), courseRepository, time.Now)
	router := authTestRouter(&authHTTP{sessions: sessions, learnerAttempts: service})
	assessmentKey := published.AssessmentBindings[0].AssessmentKey
	base := "/api/courses/by-id/" + string(course.ID) + "/versions/1.0.0/assessments/" + assessmentKey + "/attempts"
	created := authRequest(router, http.MethodPost, base, "", &http.Cookie{Name: sessionCookieName, Value: learnerSession.Token.Value()}, authTestCSRFToken().Value())
	if created.Code != http.StatusCreated || created.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create response=%d body=%s", created.Code, created.Body.String())
	}
	var attempt learnerAttemptDTO
	if err := json.Unmarshal(created.Body.Bytes(), &attempt); err != nil || attempt.State != assessments.AttemptInProgress || attempt.AssessmentKey != assessmentKey || attempt.Result != nil {
		t.Fatalf("created attempt=%#v err=%v", attempt, err)
	}
	if strings.Contains(created.Body.String(), "correctOptionKeys") || strings.Contains(created.Body.String(), "creator") || strings.Contains(created.Body.String(), string(learner.ID)) {
		t.Fatalf("created Attempt leaked private data: %s", created.Body.String())
	}

	update := learnerAttemptUpdateRequest{ExpectedRevision: pointer(attempt.Revision), Responses: []learnerAttemptResponseDTO{{QuestionKey: "publication-question", Type: assessments.QuestionSingleChoice, SelectedOptionKey: "atomic", SelectedOptionKeys: []string{}, Pairs: []learnerAttemptMatchingPairDTO{}}}}
	updated := assessmentIntegrationRequest(t, http.MethodPut, "/api/learner/assessment-attempts/"+attempt.AttemptID, learnerSession.Token.Value(), update)
	updateResponse := httptest.NewRecorder()
	router.ServeHTTP(updateResponse, updated)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update response=%d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	if err := json.Unmarshal(updateResponse.Body.Bytes(), &attempt); err != nil || attempt.Revision != 2 {
		t.Fatalf("updated attempt=%#v err=%v", attempt, err)
	}

	submit := assessmentIntegrationRequest(t, http.MethodPost, "/api/learner/assessment-attempts/"+attempt.AttemptID+"/submit", learnerSession.Token.Value(), learnerAttemptSubmitRequest{ExpectedRevision: pointer(attempt.Revision)})
	submitResponse := httptest.NewRecorder()
	router.ServeHTTP(submitResponse, submit)
	if submitResponse.Code != http.StatusOK || strings.Contains(submitResponse.Body.String(), "correctOptionKeys") {
		t.Fatalf("submit response=%d body=%s", submitResponse.Code, submitResponse.Body.String())
	}
	if err := json.Unmarshal(submitResponse.Body.Bytes(), &attempt); err != nil || attempt.State != assessments.AttemptSubmitted || attempt.Result == nil || attempt.Result.CorrectCount != 1 || attempt.Result.TotalCount != 1 || attempt.SubmittedAt == nil || attempt.SubmittedAt.Before(attempt.CreatedAt) {
		t.Fatalf("submitted attempt=%#v err=%v", attempt, err)
	}

	foreign := authRequest(router, http.MethodGet, "/api/learner/assessment-attempts/"+attempt.AttemptID, "", &http.Cookie{Name: sessionCookieName, Value: otherSession.Token.Value()})
	if foreign.Code != http.StatusNotFound || strings.Contains(foreign.Body.String(), string(learner.ID)) || strings.Contains(foreign.Body.String(), "publication-question") {
		t.Fatalf("foreign attempt response=%d body=%s", foreign.Code, foreign.Body.String())
	}
	repeated := assessmentIntegrationRequest(t, http.MethodPost, "/api/learner/assessment-attempts/"+attempt.AttemptID+"/submit", learnerSession.Token.Value(), learnerAttemptSubmitRequest{ExpectedRevision: pointer(attempt.Revision)})
	repeatedResponse := httptest.NewRecorder()
	router.ServeHTTP(repeatedResponse, repeated)
	if repeatedResponse.Code != http.StatusConflict {
		t.Fatalf("repeated submission status=%d body=%s", repeatedResponse.Code, repeatedResponse.Body.String())
	}
}
