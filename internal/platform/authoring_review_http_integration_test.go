//go:build integration

package platform

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringReviewAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	course, err := coursespostgres.New(pool).CreateCourse(ctx, "authoring-review-http-api")
	if err != nil {
		t.Fatal(err)
	}
	identityRepository := identitypostgres.New(pool)
	maintainer, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	authorUser, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	repository := authoringpostgres.New(pool)
	draft, workspace, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(maintainer.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := addTestAuthoringMember(ctx, pool, repository, workspace.ID, string(authorUser.ID), authoring.MemberAuthor); err != nil {
		t.Fatal(err)
	}
	draft, err = repository.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, courses.CourseID(course.ID)), string(maintainer.ID))
	if err != nil {
		t.Fatal(err)
	}

	sessions := identity.NewSessionService(identityRepository, nil, nil)
	authorSession, err := sessions.CreateSession(ctx, authorUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	maintainerSession, err := sessions.CreateSession(ctx, maintainer.ID)
	if err != nil {
		t.Fatal(err)
	}
	service := authoring.NewReviewApplicationService(repository, authoring.NewAuthorizationService(repository))
	router := authTestRouter(&authHTTP{sessions: sessions, authoringReviews: service})
	authorCookie := &http.Cookie{Name: sessionCookieName, Value: authorSession.Token.Value()}
	maintainerCookie := &http.Cookie{Name: sessionCookieName, Value: maintainerSession.Token.Value()}
	csrf := authTestCSRFToken().Value()
	base := "/api/authoring/drafts/" + string(draft.ID) + "/reviews"

	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":`+stringInt64(draft.Revision)+`}`, authorCookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF = %d", response.Code)
	}
	response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":`+stringInt64(draft.Revision)+`}`, authorCookie, csrf)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"status":"IN_REVIEW"`) || !strings.Contains(response.Body.String(), `"title":"Draft course"`) {
		t.Fatalf("review submit = %d %s", response.Code, response.Body.String())
	}
	var cycleID string
	marker := `"id":"`
	start := strings.Index(response.Body.String(), marker)
	if start < 0 {
		t.Fatal("review response omitted ID")
	}
	cycleID = response.Body.String()[start+len(marker):][:36]

	for _, path := range []string{base, base + "/active", base + "/latest", base + "/" + cycleID} {
		got := authRequest(router, http.MethodGet, path, "", authorCookie)
		if got.Code != http.StatusOK || got.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("AUTHOR GET %s = %d", path, got.Code)
		}
	}
	if got := authRequest(router, http.MethodPost, base+"/"+cycleID+"/approve", `{"expectedReviewRevision":1}`, authorCookie, csrf); got.Code != http.StatusNotFound {
		t.Fatalf("AUTHOR approve = %d", got.Code)
	}
	if got := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":`+stringInt64(draft.Revision)+`}`, authorCookie, csrf); got.Code != http.StatusConflict {
		t.Fatalf("duplicate active review = %d", got.Code)
	}
	if got := authRequest(router, http.MethodPost, base+"/"+cycleID+"/approve", `{"expectedReviewRevision":1}`, maintainerCookie, csrf); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"status":"APPROVED"`) {
		t.Fatalf("MAINTAINER approve = %d %s", got.Code, got.Body.String())
	}
	if got := authRequest(router, http.MethodPost, base+"/"+cycleID+"/request-changes", `{"expectedReviewRevision":1}`, maintainerCookie, csrf); got.Code != http.StatusConflict {
		t.Fatalf("terminal overwrite = %d", got.Code)
	}
	if got := authRequest(router, http.MethodGet, base+"/active", "", maintainerCookie); got.Code != http.StatusNotFound {
		t.Fatalf("terminal cycle remained active = %d", got.Code)
	}
	if got := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(otherDraft.ID)+"/reviews/"+cycleID, "", maintainerCookie); got.Code != http.StatusNotFound {
		t.Fatalf("cross-Draft review leak = %d", got.Code)
	}

	if _, err := revokeTestAuthoringMember(ctx, pool, repository, workspace.ID, string(authorUser.ID)); err != nil {
		t.Fatal(err)
	}
	if got := authRequest(router, http.MethodGet, base+"/latest", "", authorCookie); got.Code != http.StatusNotFound {
		t.Fatalf("revoked Review read = %d", got.Code)
	}
}

func stringInt64(value int64) string { return strconv.FormatInt(value, 10) }
