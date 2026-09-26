package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type publicationHTTPPublisherFake struct {
	command authoring.PublishReviewCommand
	result  authoring.PublicationResult
	err     error
	calls   int
}

func (f *publicationHTTPPublisherFake) Publish(_ context.Context, command authoring.PublishReviewCommand) (authoring.PublicationResult, error) {
	f.calls++
	f.command = command
	return f.result, f.err
}

func publicationHTTPRouter(t *testing.T, role authoring.MemberRole, publisher *publicationHTTPPublisherFake, at time.Time) http.Handler {
	t.Helper()
	draftID := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	roles := map[authoring.DraftID]authoring.MemberRole{}
	if role != "" {
		roles[draftID] = role
	}
	service := authoring.NewPublicationApplicationService(publisher, authoring.NewAuthorizationService(&authoringMembershipsFake{roles: roles}), func() time.Time { return at })
	return authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, authoringPublications: service})
}

func TestAuthoringPublicationHTTPAuthenticationAuthorizationAndTrustedMetadata(t *testing.T) {
	draftID := "11111111-1111-4111-8111-111111111111"
	reviewID := "22222222-2222-4222-8222-222222222222"
	path := "/api/authoring/drafts/" + draftID + "/reviews/" + reviewID + "/publish"
	at := time.Date(2026, time.September, 16, 19, 0, 0, 0, time.UTC)
	result := authoring.PublicationResult{
		ReviewID: authoring.ReviewID(reviewID), ReviewRevision: 2,
		CourseID: "33333333-3333-4333-8333-333333333333", CourseVersion: courses.Version{Major: 1, Minor: 2, Patch: 3},
		CourseVersionID: "44444444-4444-4444-8444-444444444444", PublishedAt: at,
	}
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()

	publisher := &publicationHTTPPublisherFake{result: result}
	router := publicationHTTPRouter(t, authoring.MemberMaintainer, publisher, at)
	if response := authRequest(router, http.MethodPost, path, `{"expectedReviewRevision":2}`, nil, csrf); response.Code != http.StatusUnauthorized || publisher.calls != 0 {
		t.Fatalf("unauthenticated response=%d calls=%d", response.Code, publisher.calls)
	}
	if response := authRequest(router, http.MethodPost, path, `{"expectedReviewRevision":2}`, cookie); response.Code != http.StatusForbidden || publisher.calls != 0 {
		t.Fatalf("missing CSRF response=%d calls=%d", response.Code, publisher.calls)
	}
	untrustedRequest := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"expectedReviewRevision":2}`))
	untrustedRequest.Header.Set("Content-Type", "application/json")
	untrustedRequest.Header.Set("Origin", "https://attacker.example")
	untrustedRequest.Header.Set("X-CSRF-Token", csrf)
	untrustedRequest.AddCookie(cookie)
	untrustedResponse := httptest.NewRecorder()
	router.ServeHTTP(untrustedResponse, untrustedRequest)
	if untrustedResponse.Code != http.StatusForbidden || publisher.calls != 0 {
		t.Fatalf("untrusted Origin response=%d calls=%d", untrustedResponse.Code, publisher.calls)
	}

	deniedPublisher := &publicationHTTPPublisherFake{result: result}
	denied := publicationHTTPRouter(t, authoring.MemberAuthor, deniedPublisher, at)
	response := authRequest(denied, http.MethodPost, path, `{"expectedReviewRevision":2}`, cookie, csrf)
	if response.Code != http.StatusNotFound || deniedPublisher.calls != 0 || strings.Contains(response.Body.String(), "permission") || strings.Contains(response.Body.String(), "MAINTAINER") {
		t.Fatalf("unauthorized response=%d calls=%d body=%s", response.Code, deniedPublisher.calls, response.Body.String())
	}

	response = authRequest(router, http.MethodPost, path, `{"expectedReviewRevision":2}`, cookie, csrf)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("publication response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	for _, expected := range []string{`"reviewId":"` + reviewID + `"`, `"reviewRevision":2`, `"courseVersion":"1.2.3"`, `"courseVersionId":"44444444-4444-4444-8444-444444444444"`, `"publishedAt":"2026-09-16T19:00:00Z"`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("publication response omitted %s: %s", expected, response.Body.String())
		}
	}
	if publisher.command.PublishedByUserID != string(loginTestUser) || !publisher.command.PublishedAt.Equal(at) || publisher.command.ExpectedReviewRevision != 2 || publisher.command.Attribution != nil {
		t.Fatalf("publication command accepted client metadata: %#v", publisher.command)
	}

	before := publisher.calls
	spoof := authRequest(router, http.MethodPost, path, `{"expectedReviewRevision":2,"publishedBy":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","publishedAt":"2000-01-01T00:00:00Z"}`, cookie, csrf)
	if spoof.Code != http.StatusBadRequest || publisher.calls != before {
		t.Fatalf("spoof response=%d calls=%d", spoof.Code, publisher.calls)
	}
}

func TestAuthoringPublicationHTTPStrictRequestAndErrorMapping(t *testing.T) {
	draftID := "11111111-1111-4111-8111-111111111111"
	reviewID := "22222222-2222-4222-8222-222222222222"
	path := "/api/authoring/drafts/" + draftID + "/reviews/" + reviewID + "/publish"
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	at := time.Date(2026, time.September, 16, 19, 0, 0, 0, time.UTC)
	publisher := &publicationHTTPPublisherFake{}
	router := publicationHTTPRouter(t, authoring.MemberMaintainer, publisher, at)

	for _, body := range []string{
		`{}`, `{"expectedReviewRevision":null}`, `{"expectedReviewRevision":0}`,
		`{"ExpectedReviewRevision":2}`, `{"expectedReviewRevision":2,"unknown":true}`,
		`{"expectedReviewRevision":2,"expectedReviewRevision":2}`, `{"expectedReviewRevision":2} {}`, `{`,
	} {
		if response := authRequest(router, http.MethodPost, path, body, cookie, csrf); response.Code != http.StatusBadRequest {
			t.Fatalf("strict body %q = %d %s", body, response.Code, response.Body.String())
		}
	}
	if response := authRequest(router, http.MethodPost, path, strings.Repeat(" ", int(maxAuthoringPublicationBodyBytes))+`{"expectedReviewRevision":2}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized body = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, "/api/authoring/drafts/not-a-uuid/reviews/"+reviewID+"/publish", `{"expectedReviewRevision":2}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed Draft ID = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, "/api/authoring/drafts/"+draftID+"/reviews/not-a-uuid/publish", `{"expectedReviewRevision":2}`, cookie, csrf); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed Review ID = %d", response.Code)
	}

	validation := &authoring.PublicationValidationFailure{Result: authoring.PublicationValidationResult{Issues: []authoring.PublicationValidationIssue{
		{Code: authoring.PublicationIssueAssetUnresolved, Path: "modules[0].lessons[0].content.blocks[0]", Message: "Asset delivery is unavailable."},
		{Code: authoring.PublicationIssueAssessmentUnresolved, Path: "modules[0].lessons[0].content.blocks[1]", Message: "Assessments are unavailable."},
	}}}
	for _, test := range []struct {
		name       string
		err        error
		status     int
		code       string
		notContain string
	}{
		{name: "not found", err: authoring.ErrReviewNotFound, status: http.StatusNotFound},
		{name: "stale", err: authoring.ErrReviewStale, status: http.StatusConflict, code: "review_revision_conflict"},
		{name: "not approved", err: authoring.ErrReviewInvalidState, status: http.StatusConflict, code: "review_not_approved"},
		{name: "validation", err: validation, status: http.StatusUnprocessableEntity, code: "publication_validation_failed"},
		{name: "foreign version", err: courses.ErrCourseVersionAlreadyExists, status: http.StatusConflict, code: "course_version_already_exists"},
		{name: "provenance", err: authoring.ErrPublicationProvenanceMismatch, status: http.StatusConflict, code: "publication_conflict"},
		{name: "infrastructure", err: errors.New("pq: constraint secret_publication_key"), status: http.StatusInternalServerError, notContain: "constraint"},
	} {
		t.Run(test.name, func(t *testing.T) {
			publisher.err = test.err
			response := authRequest(router, http.MethodPost, path, `{"expectedReviewRevision":2}`, cookie, csrf)
			if response.Code != test.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
			if test.code != "" && !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("missing code %s: %s", test.code, response.Body.String())
			}
			if test.name == "validation" && (!strings.Contains(response.Body.String(), `"code":"unresolved_asset_reference"`) || !strings.Contains(response.Body.String(), `"code":"unresolved_assessment_reference"`)) {
				t.Fatalf("validation issues flattened: %s", response.Body.String())
			}
			if test.notContain != "" && strings.Contains(response.Body.String(), test.notContain) {
				t.Fatalf("internal detail leaked: %s", response.Body.String())
			}
		})
	}
}
