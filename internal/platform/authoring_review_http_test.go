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

type authoringReviewPublicationFactsFake struct {
	record authoring.PublicationRecord
	err    error
}

func (f authoringReviewPublicationFactsFake) RecordPublication(context.Context, authoring.PublicationRecord) (authoring.PublicationRecord, error) {
	return authoring.PublicationRecord{}, errors.New("not used")
}

func (f authoringReviewPublicationFactsFake) GetPublication(context.Context, authoring.ReviewID) (authoring.PublicationRecord, error) {
	return f.record, f.err
}

type authoringReviewRepositoryFake struct {
	cycle    authoring.ReviewCycle
	snapshot authoring.ReviewSnapshot
	history  []authoring.ReviewCycle
	err      error
}

func (r *authoringReviewRepositoryFake) SubmitReview(_ context.Context, draft authoring.DraftID, expected int64, actor string) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	if r.err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, r.err
	}
	if expected != r.cycle.DraftRevision {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, authoring.ErrRevisionMismatch
	}
	r.cycle.DraftID, r.cycle.SubmittedByUserID = draft, actor
	return r.cycle, r.snapshot, nil
}
func (r *authoringReviewRepositoryFake) GetReview(context.Context, authoring.ReviewID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	return r.cycle, r.snapshot, r.err
}
func (r *authoringReviewRepositoryFake) GetReviewForDraft(_ context.Context, draft authoring.DraftID, review authoring.ReviewID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	if r.err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, r.err
	}
	if r.cycle.DraftID != draft || r.cycle.ID != review {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, authoring.ErrReviewNotFound
	}
	return r.cycle, r.snapshot, nil
}
func (r *authoringReviewRepositoryFake) LatestReview(context.Context, authoring.DraftID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	return r.cycle, r.snapshot, r.err
}
func (r *authoringReviewRepositoryFake) ActiveReview(context.Context, authoring.DraftID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	return r.cycle, r.snapshot, r.err
}
func (r *authoringReviewRepositoryFake) ListReviewHistory(context.Context, authoring.DraftID) ([]authoring.ReviewCycle, error) {
	return r.history, r.err
}
func (r *authoringReviewRepositoryFake) DecideReview(context.Context, authoring.ReviewID, int64, authoring.ReviewStatus, string, string) (authoring.ReviewCycle, error) {
	return r.cycle, r.err
}
func (r *authoringReviewRepositoryFake) DecideReviewForDraft(_ context.Context, draft authoring.DraftID, review authoring.ReviewID, expected int64, status authoring.ReviewStatus, actor, _ string) (authoring.ReviewCycle, error) {
	if r.err != nil {
		return authoring.ReviewCycle{}, r.err
	}
	if r.cycle.DraftID != draft || r.cycle.ID != review {
		return authoring.ReviewCycle{}, authoring.ErrReviewNotFound
	}
	if r.cycle.Status != authoring.ReviewInReview {
		return authoring.ReviewCycle{}, authoring.ErrReviewInvalidState
	}
	if r.cycle.Revision != expected {
		return authoring.ReviewCycle{}, authoring.ErrReviewStale
	}
	r.cycle.Status, r.cycle.Revision, r.cycle.DecidedByUserID = status, expected+1, actor
	decided := time.Now().UTC()
	r.cycle.DecidedAt = &decided
	return r.cycle, nil
}
func (r *authoringReviewRepositoryFake) ListReviewEvents(context.Context, authoring.ReviewID) ([]authoring.ReviewEvent, error) {
	return nil, r.err
}
func (r *authoringReviewRepositoryFake) ApprovedReviewForRevision(context.Context, authoring.DraftID, int64) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	return r.cycle, r.snapshot, r.err
}

func TestAuthoringReviewHTTPAuthorizationSecurityAndStrictJSON(t *testing.T) {
	draftID := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	reviewID := authoring.ReviewID("22222222-2222-4222-8222-222222222222")
	repository := &authoringReviewRepositoryFake{cycle: authoring.ReviewCycle{ID: reviewID, DraftID: draftID, DraftRevision: 4, SnapshotSchemaVersion: 1, Status: authoring.ReviewInReview, Revision: 1, SubmittedByUserID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", SubmittedAt: time.Now().UTC()}, snapshot: authoring.ReviewSnapshot{SchemaVersion: 1}, history: []authoring.ReviewCycle{}}
	memberships := authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftID: authoring.MemberAuthor}}
	service := authoring.NewReviewApplicationService(repository, authoring.NewAuthorizationService(&memberships))
	resolver := &authResolverFake{current: loginTestCurrent(t)}
	router := authTestRouter(&authHTTP{sessions: resolver, authoringReviews: service})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	base := "/api/authoring/drafts/" + string(draftID) + "/reviews"

	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4}`, nil, csrf); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated submit = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4}`, cookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF = %d", response.Code)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4}`, cookie, "wrong"); response.Code != http.StatusForbidden {
		t.Fatalf("invalid CSRF = %d", response.Code)
	}
	missingOrigin := httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{"expectedDraftRevision":4}`))
	missingOrigin.Header.Set("Content-Type", "application/json")
	missingOrigin.Header.Set("X-CSRF-Token", csrf)
	missingOrigin.AddCookie(cookie)
	missingOriginResponse := httptest.NewRecorder()
	router.ServeHTTP(missingOriginResponse, missingOrigin)
	if missingOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("missing Origin = %d", missingOriginResponse.Code)
	}
	untrusted := httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{"expectedDraftRevision":4}`))
	untrusted.Header.Set("Content-Type", "application/json")
	untrusted.Header.Set("Origin", "https://attacker.example")
	untrusted.Header.Set("X-CSRF-Token", csrf)
	untrusted.AddCookie(cookie)
	untrustedResponse := httptest.NewRecorder()
	router.ServeHTTP(untrustedResponse, untrusted)
	if untrustedResponse.Code != http.StatusForbidden {
		t.Fatalf("untrusted Origin = %d", untrustedResponse.Code)
	}
	if response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":3}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("stale Draft revision = %d", response.Code)
	}
	response := authRequest(router, http.MethodPost, base, `{"expectedDraftRevision":4}`, cookie, csrf)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"draftRevision":4`) {
		t.Fatalf("AUTHOR submit = %d %s", response.Code, response.Body.String())
	}
	if response = authRequest(router, http.MethodPost, base+"/"+string(reviewID)+"/approve", `{"expectedReviewRevision":1}`, cookie, csrf); response.Code != http.StatusNotFound {
		t.Fatalf("AUTHOR decision = %d", response.Code)
	}
	for _, suffix := range []string{"approve", "request-changes"} {
		decisionPath := base + "/" + string(reviewID) + "/" + suffix
		if got := authRequest(router, http.MethodPost, decisionPath, `{"expectedReviewRevision":1}`, nil, csrf); got.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s = %d", suffix, got.Code)
		}
		if got := authRequest(router, http.MethodPost, decisionPath, `{"expectedReviewRevision":1}`, cookie); got.Code != http.StatusForbidden {
			t.Fatalf("missing CSRF %s = %d", suffix, got.Code)
		}
		if got := authRequest(router, http.MethodPost, decisionPath, `{"expectedReviewRevision":1}`, cookie, "wrong"); got.Code != http.StatusForbidden {
			t.Fatalf("invalid CSRF %s = %d", suffix, got.Code)
		}
		missingOriginDecision := httptest.NewRequest(http.MethodPost, decisionPath, strings.NewReader(`{"expectedReviewRevision":1}`))
		missingOriginDecision.Header.Set("Content-Type", "application/json")
		missingOriginDecision.Header.Set("X-CSRF-Token", csrf)
		missingOriginDecision.AddCookie(cookie)
		missingOriginDecisionResponse := httptest.NewRecorder()
		router.ServeHTTP(missingOriginDecisionResponse, missingOriginDecision)
		if missingOriginDecisionResponse.Code != http.StatusForbidden {
			t.Fatalf("missing Origin %s = %d", suffix, missingOriginDecisionResponse.Code)
		}
		untrustedDecision := httptest.NewRequest(http.MethodPost, decisionPath, strings.NewReader(`{"expectedReviewRevision":1}`))
		untrustedDecision.Header.Set("Content-Type", "application/json")
		untrustedDecision.Header.Set("Origin", "https://attacker.example")
		untrustedDecision.Header.Set("X-CSRF-Token", csrf)
		untrustedDecision.AddCookie(cookie)
		untrustedDecisionResponse := httptest.NewRecorder()
		router.ServeHTTP(untrustedDecisionResponse, untrustedDecision)
		if untrustedDecisionResponse.Code != http.StatusForbidden {
			t.Fatalf("untrusted Origin %s = %d", suffix, untrustedDecisionResponse.Code)
		}
	}

	// The remaining decision transport checks model an independent maintainer.
	repository.cycle.SubmittedByUserID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	memberships.roles[draftID] = authoring.MemberMaintainer
	for _, suffix := range []string{"approve", "request-changes"} {
		decisionPath := base + "/" + string(reviewID) + "/" + suffix
		for _, body := range []string{
			`{}`,
			`{"expectedReviewRevision":null}`,
			`{"ExpectedReviewRevision":1}`,
			`{"expectedReviewRevision":1,"unknown":true}`,
			`{"expectedReviewRevision":1,"expectedReviewRevision":1}`,
			`{"expectedReviewRevision":1} {}`,
			`{`,
		} {
			if got := authRequest(router, http.MethodPost, decisionPath, body, cookie, csrf); got.Code != http.StatusBadRequest {
				t.Fatalf("strict %s body %q = %d", suffix, body, got.Code)
			}
		}
		if got := authRequest(router, http.MethodPost, decisionPath, strings.Repeat(" ", int(maxAuthoringReviewBodyBytes))+`{"expectedReviewRevision":1}`, cookie, csrf); got.Code != http.StatusBadRequest {
			t.Fatalf("oversized %s body = %d", suffix, got.Code)
		}
	}
	response = authRequest(router, http.MethodPost, base+"/"+string(reviewID)+"/approve", `{"expectedReviewRevision":1,"message":"Approved"}`, cookie, csrf)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"APPROVED"`) {
		t.Fatalf("MAINTAINER approve = %d %s", response.Code, response.Body.String())
	}
	if response = authRequest(router, http.MethodPost, base+"/"+string(reviewID)+"/request-changes", `{"expectedReviewRevision":1}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("terminal re-decision = %d", response.Code)
	}
	repository.cycle.Status, repository.cycle.Revision, repository.cycle.DecidedAt, repository.cycle.DecidedByUserID = authoring.ReviewInReview, 1, nil, ""
	repository.cycle.Revision = 2
	if response = authRequest(router, http.MethodPost, base+"/"+string(reviewID)+"/request-changes", `{"expectedReviewRevision":1}`, cookie, csrf); response.Code != http.StatusConflict {
		t.Fatalf("stale Review revision = %d", response.Code)
	}
	repository.cycle.Revision = 1
	if response = authRequest(router, http.MethodPost, base+"/"+string(reviewID)+"/request-changes", `{"expectedReviewRevision":1,"message":"Please revise"}`, cookie, csrf); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"CHANGES_REQUESTED"`) {
		t.Fatalf("MAINTAINER request changes = %d %s", response.Code, response.Body.String())
	}

	if response = authRequest(router, http.MethodGet, base+"/not-a-uuid", "", cookie); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed review ID = %d", response.Code)
	}
	if response = authRequest(router, http.MethodGet, "/api/authoring/drafts/not-a-uuid/reviews/latest", "", cookie); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed Draft ID = %d", response.Code)
	}
	otherDraft := "33333333-3333-4333-8333-333333333333"
	memberships.roles[authoring.DraftID(otherDraft)] = authoring.MemberMaintainer
	if response = authRequest(router, http.MethodGet, "/api/authoring/drafts/"+otherDraft+"/reviews/"+string(reviewID), "", cookie); response.Code != http.StatusNotFound {
		t.Fatalf("cross-Draft review = %d", response.Code)
	}
	for _, suffix := range []string{"approve", "request-changes"} {
		response = authRequest(router, http.MethodPost, "/api/authoring/drafts/"+otherDraft+"/reviews/"+string(reviewID)+"/"+suffix, `{"expectedReviewRevision":1}`, cookie, csrf)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) {
			t.Fatalf("cross-Draft %s policy leak = %d %s", suffix, response.Code, response.Body.String())
		}
	}
	delete(memberships.roles, draftID)
	if response = authRequest(router, http.MethodGet, base+"/latest", "", cookie); response.Code != http.StatusNotFound {
		t.Fatalf("revoked read = %d", response.Code)
	}
	memberships.err = errors.New("database unavailable")
	if response = authRequest(router, http.MethodGet, base+"/latest", "", cookie); response.Code != http.StatusInternalServerError {
		t.Fatalf("authorization failure = %d", response.Code)
	}
}

func TestAuthoringReviewHTTPReadsExcludeSnapshotsFromHistory(t *testing.T) {
	draftID := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	reviewID := authoring.ReviewID("22222222-2222-4222-8222-222222222222")
	cycle := authoring.ReviewCycle{ID: reviewID, DraftID: draftID, DraftRevision: 4, SnapshotSchemaVersion: 1, Status: authoring.ReviewInReview, Revision: 1, SubmittedByUserID: string(loginTestUser), SubmittedAt: time.Now().UTC()}
	repository := &authoringReviewRepositoryFake{cycle: cycle, snapshot: authoring.ReviewSnapshot{SchemaVersion: 1}, history: []authoring.ReviewCycle{cycle}}
	service := authoring.NewReviewApplicationService(repository, authoring.NewAuthorizationService(authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftID: authoring.MemberAuthor}}))
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, authoringReviews: service})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	base := "/api/authoring/drafts/" + string(draftID) + "/reviews"
	for _, path := range []string{base, base + "/active", base + "/latest", base + "/" + string(reviewID)} {
		response := authRequest(router, http.MethodGet, path, "", cookie)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("GET %s = %d", path, response.Code)
		}
		if path != base+"/"+string(reviewID) && strings.Contains(response.Body.String(), `"snapshot"`) {
			t.Fatalf("metadata read included snapshot: %s", path)
		}
	}
}

func TestAuthoringReviewHTTPIncludesSafeExactPublicationStatus(t *testing.T) {
	draftID := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	reviewID := authoring.ReviewID("22222222-2222-4222-8222-222222222222")
	version, err := courses.ParseVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	cycle := authoring.ReviewCycle{ID: reviewID, DraftID: draftID, DraftRevision: 4, SnapshotSchemaVersion: 1, Status: authoring.ReviewApproved, Revision: 2, SubmittedByUserID: string(loginTestUser), SubmittedAt: time.Now().UTC()}
	repository := &authoringReviewRepositoryFake{cycle: cycle, snapshot: authoring.ReviewSnapshot{SchemaVersion: 1}}
	memberships := authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftID: authoring.MemberMaintainer}}
	authorizer := authoring.NewAuthorizationService(memberships)
	facts := authoringReviewPublicationFactsFake{record: authoring.PublicationRecord{
		ReviewID: reviewID, ReviewRevision: cycle.Revision, DraftID: draftID, DraftRevision: cycle.DraftRevision,
		CourseID: "33333333-3333-4333-8333-333333333333", CourseVersion: version,
		CourseVersionID: "44444444-4444-4444-8444-444444444444", PublishedAt: publishedAt,
		PublishedByUserID: "55555555-5555-4555-8555-555555555555",
	}}
	router := authTestRouter(&authHTTP{
		sessions:                   &authResolverFake{current: loginTestCurrent(t)},
		authoringReviews:           authoring.NewReviewApplicationService(repository, authorizer),
		authoringPublicationStatus: authoring.NewReviewPublicationStatusService(facts, authorizer),
	})
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	response := authRequest(router, http.MethodGet, "/api/authoring/drafts/"+string(draftID)+"/reviews/"+string(reviewID), "", cookie)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("publication status = %d %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{`"canPublish":true`, `"courseVersion":"1.0.0"`, `"publishedAt":"2026-09-17T12:00:00Z"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
	for _, forbidden := range []string{"publishedBy", "courseVersionId", "55555555-5555-4555-8555-555555555555"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("publication projection exposed %q: %s", forbidden, body)
		}
	}
}

func TestAuthoringReviewHTTPIndependentReviewerPolicyConflict(t *testing.T) {
	draftID := authoring.DraftID("11111111-1111-4111-8111-111111111111")
	reviewID := authoring.ReviewID("22222222-2222-4222-8222-222222222222")
	cookie := &http.Cookie{Name: sessionCookieName, Value: mustRawToken().Value()}
	csrf := authTestCSRFToken().Value()
	base := "/api/authoring/drafts/" + string(draftID) + "/reviews/" + string(reviewID)
	newRouter := func(policy authoring.ReviewDecisionPolicy, status authoring.ReviewStatus, revision int64, submittedBy string, role authoring.MemberRole) (*authoringReviewRepositoryFake, *authoringMembershipsFake, http.Handler) {
		repository := &authoringReviewRepositoryFake{cycle: authoring.ReviewCycle{ID: reviewID, DraftID: draftID, Status: status, Revision: revision, SubmittedByUserID: submittedBy, SubmittedAt: time.Now().UTC()}}
		memberships := &authoringMembershipsFake{roles: map[authoring.DraftID]authoring.MemberRole{draftID: role}}
		service := authoring.NewReviewApplicationServiceWithDecisionPolicy(repository, authoring.NewAuthorizationService(memberships), policy)
		return repository, memberships, authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, authoringReviews: service})
	}

	for _, suffix := range []string{"approve", "request-changes"} {
		t.Run("enabled self "+suffix, func(t *testing.T) {
			repository, _, router := newRouter(authoring.NewReviewDecisionPolicy(true), authoring.ReviewInReview, 1, string(loginTestUser), authoring.MemberMaintainer)
			response := authRequest(router, http.MethodPost, base+"/"+suffix, `{"expectedReviewRevision":1}`, cookie, csrf)
			if response.Code != http.StatusConflict || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) || !strings.Contains(response.Body.String(), `"title":"Independent reviewer required"`) {
				t.Fatalf("policy conflict = %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), string(loginTestUser)) || strings.Contains(response.Body.String(), "submittedBy") || repository.cycle.Status != authoring.ReviewInReview || repository.cycle.Revision != 1 {
				t.Fatalf("policy conflict exposed or mutated Review: %s %#v", response.Body.String(), repository.cycle)
			}
		})
	}

	for _, suffix := range []string{"approve", "request-changes"} {
		t.Run("disabled self "+suffix, func(t *testing.T) {
			repository, _, router := newRouter(authoring.NewReviewDecisionPolicy(false), authoring.ReviewInReview, 1, string(loginTestUser), authoring.MemberMaintainer)
			response := authRequest(router, http.MethodPost, base+"/"+suffix, `{"expectedReviewRevision":1}`, cookie, csrf)
			if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) || repository.cycle.Revision != 2 {
				t.Fatalf("disabled policy decision = %d %s", response.Code, response.Body.String())
			}
		})
	}

	for _, suffix := range []string{"approve", "request-changes"} {
		t.Run("stale "+suffix+" remains generic conflict", func(t *testing.T) {
			_, _, router := newRouter(authoring.NewReviewDecisionPolicy(true), authoring.ReviewInReview, 1, string(loginTestUser), authoring.MemberMaintainer)
			response := authRequest(router, http.MethodPost, base+"/"+suffix, `{"expectedReviewRevision":2}`, cookie, csrf)
			if response.Code != http.StatusConflict || strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) {
				t.Fatalf("stale conflict = %d %s", response.Code, response.Body.String())
			}
		})
		t.Run("terminal "+suffix+" remains generic conflict", func(t *testing.T) {
			_, _, router := newRouter(authoring.NewReviewDecisionPolicy(true), authoring.ReviewApproved, 1, string(loginTestUser), authoring.MemberMaintainer)
			response := authRequest(router, http.MethodPost, base+"/"+suffix, `{"expectedReviewRevision":1}`, cookie, csrf)
			if response.Code != http.StatusConflict || strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) {
				t.Fatalf("terminal conflict = %d %s", response.Code, response.Body.String())
			}
		})
	}

	for _, suffix := range []string{"approve", "request-changes"} {
		t.Run("unauthorized "+suffix+" remains hidden", func(t *testing.T) {
			_, _, router := newRouter(authoring.NewReviewDecisionPolicy(true), authoring.ReviewInReview, 1, string(loginTestUser), authoring.MemberAuthor)
			response := authRequest(router, http.MethodPost, base+"/"+suffix, `{"expectedReviewRevision":1}`, cookie, csrf)
			if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) {
				t.Fatalf("authorization behavior changed = %d %s", response.Code, response.Body.String())
			}
		})
	}

	for _, suffix := range []string{"approve", "request-changes"} {
		t.Run("revoked "+suffix+" remains hidden", func(t *testing.T) {
			_, memberships, router := newRouter(authoring.NewReviewDecisionPolicy(true), authoring.ReviewInReview, 1, string(loginTestUser), authoring.MemberMaintainer)
			delete(memberships.roles, draftID)
			response := authRequest(router, http.MethodPost, base+"/"+suffix, `{"expectedReviewRevision":1}`, cookie, csrf)
			if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) {
				t.Fatalf("revoked behavior changed = %d %s", response.Code, response.Body.String())
			}
		})
		t.Run("storage failure "+suffix+" remains unavailable", func(t *testing.T) {
			repository, _, router := newRouter(authoring.NewReviewDecisionPolicy(true), authoring.ReviewInReview, 1, "other-user", authoring.MemberMaintainer)
			repository.err = errors.New("review storage unavailable")
			response := authRequest(router, http.MethodPost, base+"/"+suffix, `{"expectedReviewRevision":1}`, cookie, csrf)
			if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), `"code":"independent_reviewer_required"`) {
				t.Fatalf("storage failure behavior changed = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
