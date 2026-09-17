//go:build integration

package platform

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type failOncePublicationRepository struct {
	base authoring.PublicationRecordRepository
	mu   sync.Mutex
	fail bool
}

func (r *failOncePublicationRepository) RecordPublication(ctx context.Context, record authoring.PublicationRecord) (authoring.PublicationRecord, error) {
	r.mu.Lock()
	if r.fail {
		r.fail = false
		r.mu.Unlock()
		return authoring.PublicationRecord{}, errors.New("injected Authoring publication failure")
	}
	r.mu.Unlock()
	return r.base.RecordPublication(ctx, record)
}
func (r *failOncePublicationRepository) GetPublication(ctx context.Context, review authoring.ReviewID) (authoring.PublicationRecord, error) {
	return r.base.GetPublication(ctx, review)
}

func testPublicationOrchestration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	authoringRepository := authoringpostgres.New(pool)
	coursesRepository := coursespostgres.New(pool)
	courseStore := courses.NewCourseVersionStore(coursesRepository)
	course, err := coursesRepository.CreateCourse(ctx, "publication-orchestration")
	if err != nil {
		t.Fatal(err)
	}
	submitter := "d1000000-0000-4000-8000-000000000001"
	reviewer := "d1000000-0000-4000-8000-000000000002"
	publisher := "d1000000-0000-4000-8000-000000000003"
	attribution := []courses.ContributorSnapshot{{UserID: submitter, DisplayName: "Course author", Role: courses.ContributorAuthor, Order: 0}}

	cycle := createApprovedPublicationReview(t, ctx, authoringRepository, course.ID, "1.0.0", submitter, reviewer)
	beforeDraft, err := authoringRepository.GetDraft(ctx, cycle.DraftID)
	if err != nil {
		t.Fatal(err)
	}
	beforeCycle, beforeSnapshot, err := authoringRepository.GetReviewForDraft(ctx, cycle.DraftID, cycle.ID)
	if err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, time.September, 16, 15, 0, 0, 0, time.UTC)
	command := authoring.PublishReviewCommand{DraftID: cycle.DraftID, ReviewID: cycle.ID, ExpectedReviewRevision: cycle.Revision, PublishedByUserID: publisher, PublishedAt: publishedAt, Attribution: attribution}

	// Courses succeeds, the injected Authoring fact write fails, then retry
	// discovers the exact Review provenance and completes the missing fact.
	failingFacts := &failOncePublicationRepository{base: authoringRepository, fail: true}
	service := authoring.NewPublicationService(authoringRepository, courseStore, failingFacts)
	if _, err := service.Publish(ctx, command); err == nil {
		t.Fatal("injected publication-record failure did not escape")
	}
	persisted, err := courseStore.GetByReviewID(ctx, string(cycle.ID))
	if err != nil {
		t.Fatalf("Courses aggregate was rolled back after Authoring failure: %v", err)
	}
	if _, err := authoringRepository.GetPublication(ctx, cycle.ID); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("failed fact write became visible: %v", err)
	}
	recovered, err := service.Publish(ctx, command)
	if err != nil || !recovered.Reconciled || recovered.CourseVersionID != persisted.ID {
		t.Fatalf("publication retry did not reconcile: %#v %v", recovered, err)
	}
	fact, err := authoringRepository.GetPublication(ctx, cycle.ID)
	if err != nil || fact.CourseVersionID != persisted.ID || fact.PublishedByUserID != publisher || !fact.PublishedAt.Equal(publishedAt) {
		t.Fatalf("publication fact provenance: %#v %v", fact, err)
	}
	afterDraft, err := authoringRepository.GetDraft(ctx, cycle.DraftID)
	if err != nil {
		t.Fatal(err)
	}
	afterCycle, afterSnapshot, err := authoringRepository.GetReviewForDraft(ctx, cycle.DraftID, cycle.ID)
	if err != nil || !reflect.DeepEqual(beforeDraft, afterDraft) || !reflect.DeepEqual(beforeCycle, afterCycle) || !reflect.DeepEqual(beforeSnapshot, afterSnapshot) {
		t.Fatalf("publication mutated Authoring source state: %v", err)
	}
	assertPublicationCounts(t, ctx, pool, course.ID, 1, 1)

	// A different Review targeting the same logical course/version is a true
	// conflict and must never be marked as published.
	conflicting := createApprovedPublicationReview(t, ctx, authoringRepository, course.ID, "1.0.0", submitter, reviewer)
	conflictCommand := command
	conflictCommand.DraftID, conflictCommand.ReviewID, conflictCommand.ExpectedReviewRevision = conflicting.DraftID, conflicting.ID, conflicting.Revision
	if _, err := authoring.NewPublicationService(authoringRepository, courseStore, authoringRepository).Publish(ctx, conflictCommand); !errors.Is(err, courses.ErrCourseVersionAlreadyExists) {
		t.Fatalf("foreign Review SemVer conflict = %v", err)
	}
	if _, err := authoringRepository.GetPublication(ctx, conflicting.ID); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("conflicting Review acquired publication fact: %v", err)
	}

	// Concurrent publication relies on Courses and Authoring constraints, not a
	// process mutex. Both calls converge on one aggregate and one fact.
	concurrent := createApprovedPublicationReview(t, ctx, authoringRepository, course.ID, "2.0.0", submitter, reviewer)
	concurrentCommand := command
	concurrentCommand.DraftID, concurrentCommand.ReviewID, concurrentCommand.ExpectedReviewRevision = concurrent.DraftID, concurrent.ID, concurrent.Revision
	concurrentCommand.PublishedAt = publishedAt.Add(time.Hour)
	concurrentService := authoring.NewPublicationService(authoringRepository, courseStore, authoringRepository)
	barrier := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-barrier
			_, publishErr := concurrentService.Publish(ctx, concurrentCommand)
			results <- publishErr
		}()
	}
	close(barrier)
	wait.Wait()
	close(results)
	for publishErr := range results {
		if publishErr != nil {
			t.Fatalf("concurrent publication did not converge: %v", publishErr)
		}
	}
	assertPublicationCounts(t, ctx, pool, course.ID, 2, 2)
}

func testAuthoringPublicationAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	authoringRepository := authoringpostgres.New(pool)
	coursesRepository := coursespostgres.New(pool)
	courseStore := courses.NewCourseVersionStore(coursesRepository)
	identityRepository := identitypostgres.New(pool)
	publisher, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	unauthorized, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	submitter, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	course, err := coursesRepository.CreateCourse(ctx, "publication-http-api")
	if err != nil {
		t.Fatal(err)
	}
	reviewer := "d2000000-0000-4000-8000-000000000004"
	cycle := createApprovedPublicationReview(t, ctx, authoringRepository, course.ID, "3.2.1", string(submitter.ID), reviewer)
	workspace, err := authoringRepository.GetWorkspace(ctx, cycle.DraftID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, workspace.ID, string(publisher.ID), authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, workspace.ID, string(unauthorized.ID), authoring.MemberAuthor); err != nil {
		t.Fatal(err)
	}

	sessions := identity.NewSessionService(identityRepository, nil, nil)
	publisherSession, err := sessions.CreateSession(ctx, publisher.ID)
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedSession, err := sessions.CreateSession(ctx, unauthorized.ID)
	if err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, time.September, 16, 20, 0, 0, 0, time.UTC)
	failingFacts := &failOncePublicationRepository{base: authoringRepository, fail: true}
	publication := authoring.NewPublicationService(authoringRepository, courseStore, failingFacts)
	application := authoring.NewPublicationApplicationService(publication, authoring.NewAuthorizationService(authoringRepository), func() time.Time { return publishedAt })
	router := authTestRouter(&authHTTP{sessions: sessions, authoringPublications: application})
	path := "/api/authoring/drafts/" + string(cycle.DraftID) + "/reviews/" + string(cycle.ID) + "/publish"
	body := `{"expectedReviewRevision":` + stringInt64(cycle.Revision) + `}`
	csrf := authTestCSRFToken().Value()
	publisherCookie := &http.Cookie{Name: sessionCookieName, Value: publisherSession.Token.Value()}
	unauthorizedCookie := &http.Cookie{Name: sessionCookieName, Value: unauthorizedSession.Token.Value()}

	denied := authRequest(router, http.MethodPost, path, body, unauthorizedCookie, csrf)
	if denied.Code != http.StatusNotFound {
		t.Fatalf("AUTHOR publication = %d %s", denied.Code, denied.Body.String())
	}
	assertPublicationCounts(t, ctx, pool, course.ID, 0, 0)

	// Courses commits while the injected Authoring fact fails. The HTTP retry
	// reconciles exact Review provenance and remains an ordinary success.
	first := authRequest(router, http.MethodPost, path, body, publisherCookie, csrf)
	if first.Code != http.StatusInternalServerError || strings.Contains(first.Body.String(), "injected") {
		t.Fatalf("partial completion response = %d %s", first.Code, first.Body.String())
	}
	assertPublicationCounts(t, ctx, pool, course.ID, 1, 0)
	recovered := authRequest(router, http.MethodPost, path, body, publisherCookie, csrf)
	if recovered.Code != http.StatusOK || !strings.Contains(recovered.Body.String(), `"courseVersion":"3.2.1"`) || !strings.Contains(recovered.Body.String(), `"publishedAt":"2026-09-16T20:00:00Z"`) {
		t.Fatalf("reconciled publication = %d %s", recovered.Code, recovered.Body.String())
	}
	exactReplay := authRequest(router, http.MethodPost, path, body, publisherCookie, csrf)
	if exactReplay.Code != http.StatusOK {
		t.Fatalf("exact replay = %d %s", exactReplay.Code, exactReplay.Body.String())
	}
	assertPublicationCounts(t, ctx, pool, course.ID, 1, 1)
	fact, err := authoringRepository.GetPublication(ctx, cycle.ID)
	if err != nil || fact.PublishedByUserID != string(publisher.ID) || !fact.PublishedAt.Equal(publishedAt) {
		t.Fatalf("server publication provenance = %#v %v", fact, err)
	}
	stored, err := courseStore.GetByReviewID(ctx, string(cycle.ID))
	if err != nil || len(stored.CourseVersion.Attribution) != 1 || stored.CourseVersion.Attribution[0].UserID != string(submitter.ID) || stored.CourseVersion.Attribution[0].DisplayName != "Author" {
		t.Fatalf("server-owned frozen attribution = %#v %v", stored.CourseVersion.Attribution, err)
	}

	stale := authRequest(router, http.MethodPost, path, `{"expectedReviewRevision":1}`, publisherCookie, csrf)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"code":"review_revision_conflict"`) {
		t.Fatalf("stale publication = %d %s", stale.Code, stale.Body.String())
	}

	conflicting := createApprovedPublicationReview(t, ctx, authoringRepository, course.ID, "3.2.1", string(submitter.ID), reviewer)
	conflictingWorkspace, err := authoringRepository.GetWorkspace(ctx, conflicting.DraftID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, conflictingWorkspace.ID, string(publisher.ID), authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	conflictPath := "/api/authoring/drafts/" + string(conflicting.DraftID) + "/reviews/" + string(conflicting.ID) + "/publish"
	conflictBody := `{"expectedReviewRevision":` + stringInt64(conflicting.Revision) + `}`
	conflict := authRequest(router, http.MethodPost, conflictPath, conflictBody, publisherCookie, csrf)
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"code":"course_version_already_exists"`) {
		t.Fatalf("foreign SemVer conflict = %d %s", conflict.Code, conflict.Body.String())
	}
	if _, err := authoringRepository.GetPublication(ctx, conflicting.ID); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("foreign conflict recorded a publication fact: %v", err)
	}
}

func createApprovedPublicationReview(t *testing.T, ctx context.Context, repository *authoringpostgres.Repository, courseID courses.CourseID, versionText, submitter, reviewer string) authoring.ReviewCycle {
	t.Helper()
	metadata := draftFixture(t, courseID)
	version, err := courses.ParseVersion(versionText)
	if err != nil {
		t.Fatal(err)
	}
	metadata.IntendedVersion = version
	draft, _, err := repository.CreateDraft(ctx, metadata, submitter)
	if err != nil {
		t.Fatal(err)
	}
	module, draft, err := repository.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "publication-module", Title: "Publication module", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	lesson := lessonFixture(draft.ID, module.ID, "publication-lesson", 0)
	lesson.Content = courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "publication-divider", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}}
	_, draft, err = repository.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, lesson)
	if err != nil {
		t.Fatal(err)
	}
	cycle, _, err := repository.SubmitReview(ctx, draft.ID, draft.Revision, submitter)
	if err != nil {
		t.Fatal(err)
	}
	cycle, err = repository.DecideReview(ctx, cycle.ID, cycle.Revision, authoring.ReviewApproved, reviewer, "")
	if err != nil {
		t.Fatal(err)
	}
	return cycle
}

func assertPublicationCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, courseID courses.CourseID, versions, facts int) {
	t.Helper()
	var versionCount, factCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM courses.course_version WHERE course_id=$1`, courseID).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM authoring.review_publication WHERE course_id=$1`, courseID).Scan(&factCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != versions || factCount != facts {
		t.Fatalf("publication counts: versions=%d facts=%d want=%d/%d", versionCount, factCount, versions, facts)
	}
}
