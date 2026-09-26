//go:build integration

package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	assessmentspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	assetslocal "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/localstorage"
	assetspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPublicationAssessmentBinding(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	assessmentRepository := assessmentspostgres.New(pool)
	authoringRepository := authoringpostgres.New(pool).WithAssessmentSnapshotReader(func(tx pgx.Tx) authoringpostgres.AssessmentSnapshotReader { return assessmentspostgres.New(tx) })
	coursesRepository := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(coursesRepository)
	course, err := coursesRepository.CreateCourse(ctx, "publication-assessment-binding")
	if err != nil {
		t.Fatal(err)
	}
	submitter := "ab000000-0000-4000-8000-000000000001"
	reviewer := "ab000000-0000-4000-8000-000000000002"
	publisher := "ab000000-0000-4000-8000-000000000003"
	draft, _, err := authoringRepository.CreateDraft(ctx, draftFixture(t, course.ID), submitter)
	if err != nil {
		t.Fatal(err)
	}
	definitionA := []assessments.Question{{StableKey: "question", Type: assessments.QuestionSingleChoice, Prompt: "Reviewed wording A", Position: 0, Options: []assessments.ChoiceOption{{StableKey: "correct", Text: "Correct A", Position: 0}, {StableKey: "other", Text: "Other", Position: 1}}, CorrectOptionKeys: []string{"correct"}}}
	assessment, err := assessmentRepository.CreateAssessment(ctx, assessments.AssessmentInput{OwnerDraftID: string(draft.ID), Title: "Review check", Questions: definitionA, CreatedByUserID: submitter})
	if err != nil {
		t.Fatal(err)
	}
	module, draft, err := authoringRepository.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "checks", Title: "Checks", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	lesson := lessonFixture(draft.ID, module.ID, "check-lesson", 0)
	lesson.Content = courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: string(assessment.ID)}}}}
	_, draft, err = authoringRepository.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, lesson)
	if err != nil {
		t.Fatal(err)
	}
	cycle, submittedSnapshot, err := authoringRepository.SubmitReview(ctx, draft.ID, draft.Revision, submitter)
	if err != nil || len(submittedSnapshot.Assessments) != 1 || submittedSnapshot.Assessments[0].Questions[0].Prompt != "Reviewed wording A" {
		t.Fatalf("frozen Review Assessment = %#v, %v", submittedSnapshot.Assessments, err)
	}
	definitionB := cloneAssessmentDefinition(definitionA)
	definitionB[0].Prompt = "Mutable wording B"
	definitionB[0].Options[0].Text = "Correct B"
	assessment, err = assessmentRepository.UpdateAssessment(ctx, assessment.ID, assessment.Revision, assessments.AssessmentUpdate{Title: assessment.Title, Questions: definitionB})
	if err != nil {
		t.Fatal(err)
	}
	_, reloadedSnapshot, err := authoringRepository.GetReviewForDraft(ctx, draft.ID, cycle.ID)
	if err != nil || !reflect.DeepEqual(reloadedSnapshot.Assessments, submittedSnapshot.Assessments) {
		t.Fatalf("mutable edit changed Review snapshot: %#v, %v", reloadedSnapshot.Assessments, err)
	}
	cycle, err = authoringRepository.DecideReview(ctx, cycle.ID, cycle.Revision, authoring.ReviewApproved, reviewer, "")
	if err != nil {
		t.Fatal(err)
	}
	command := authoring.PublishReviewCommand{DraftID: draft.ID, ReviewID: cycle.ID, ExpectedReviewRevision: cycle.Revision, PublishedByUserID: publisher, PublishedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)}
	failingFacts := &failOncePublicationRepository{base: authoringRepository, fail: true}
	service := authoring.NewPublicationService(authoringRepository, store, failingFacts)
	if _, err := service.Publish(ctx, command); err == nil {
		t.Fatal("injected fact failure did not occur")
	}
	persisted, err := store.GetByReviewID(ctx, string(cycle.ID))
	if err != nil || len(persisted.AssessmentBindings) != 1 || persisted.AssessmentBindings[0].Questions[0].Prompt != "Reviewed wording A" || persisted.AssessmentBindings[0].Questions[0].Options[0].Text != "Correct A" {
		t.Fatalf("published binding did not preserve reviewed A: %#v, %v", persisted.AssessmentBindings, err)
	}
	definitionC := cloneAssessmentDefinition(definitionB)
	definitionC[0].Prompt = "Mutable wording C"
	if _, err := assessmentRepository.UpdateAssessment(ctx, assessment.ID, assessment.Revision, assessments.AssessmentUpdate{Title: assessment.Title, Questions: definitionC}); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.Publish(ctx, command)
	if err != nil || !recovered.Reconciled || recovered.CourseVersionID != persisted.ID {
		t.Fatalf("recovery = %#v, %v", recovered, err)
	}
	reloaded, err := store.GetByReviewID(ctx, string(cycle.ID))
	if err != nil || !reflect.DeepEqual(reloaded.AssessmentBindings, persisted.AssessmentBindings) {
		t.Fatalf("recovery reinterpreted mutable Assessment: %#v, %v", reloaded.AssessmentBindings, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM courses.course_version_assessment_binding WHERE course_version_id=$1`, persisted.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("Assessment binding count = %d, %v", count, err)
	}

	foreignCourse, err := coursesRepository.CreateCourse(ctx, "publication-foreign-assessment")
	if err != nil {
		t.Fatal(err)
	}
	foreignDraft, _, err := authoringRepository.CreateDraft(ctx, draftFixture(t, foreignCourse.ID), submitter)
	if err != nil {
		t.Fatal(err)
	}
	foreignModule, foreignDraft, err := authoringRepository.CreateModuleAtPosition(ctx, foreignDraft.ID, foreignDraft.Revision, authoring.ModuleInput{DraftID: foreignDraft.ID, StableKey: "checks", Title: "Checks", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	foreignLesson := lessonFixture(foreignDraft.ID, foreignModule.ID, "foreign-check", 0)
	foreignLesson.Content = courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "check", Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: string(assessment.ID)}}}}
	_, foreignDraft, err = authoringRepository.CreateLessonAtPosition(ctx, foreignDraft.ID, foreignModule.ID, foreignDraft.Revision, foreignLesson)
	if err != nil {
		t.Fatal(err)
	}
	foreignCycle, foreignSnapshot, err := authoringRepository.SubmitReview(ctx, foreignDraft.ID, foreignDraft.Revision, submitter)
	if err != nil || len(foreignSnapshot.Assessments) != 0 {
		t.Fatalf("foreign Assessment was frozen into Review: %#v, %v", foreignSnapshot.Assessments, err)
	}
	foreignCycle, err = authoringRepository.DecideReview(ctx, foreignCycle.ID, foreignCycle.Revision, authoring.ReviewApproved, reviewer, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = authoring.NewPublicationService(authoringRepository, store, authoringRepository).Publish(ctx, authoring.PublishReviewCommand{DraftID: foreignDraft.ID, ReviewID: foreignCycle.ID, ExpectedReviewRevision: foreignCycle.Revision, PublishedByUserID: publisher, PublishedAt: command.PublishedAt.Add(time.Hour)})
	var validation *authoring.PublicationValidationFailure
	if !errors.As(err, &validation) || !hasIntegrationPublicationIssue(validation.Result, authoring.PublicationIssueAssessmentUnavailable) {
		t.Fatalf("foreign Assessment publication error = %#v, %v", validation, err)
	}
}

func hasIntegrationPublicationIssue(result authoring.PublicationValidationResult, code authoring.PublicationValidationCode) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func cloneAssessmentDefinition(source []assessments.Question) []assessments.Question {
	result := append([]assessments.Question(nil), source...)
	for index := range result {
		result[index].Options = append([]assessments.ChoiceOption(nil), source[index].Options...)
		result[index].CorrectOptionKeys = append([]string(nil), source[index].CorrectOptionKeys...)
		result[index].LeftItems = append([]assessments.MatchingItem(nil), source[index].LeftItems...)
		result[index].RightItems = append([]assessments.MatchingItem(nil), source[index].RightItems...)
		result[index].CorrectPairs = append([]assessments.MatchingPair(nil), source[index].CorrectPairs...)
	}
	return result
}

func testPublicationAssetBinding(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	authoringRepository := authoringpostgres.New(pool)
	assetRepository := assetspostgres.New(pool)
	coursesRepository := coursespostgres.New(pool)
	courseStore := courses.NewCourseVersionStore(coursesRepository)
	course, err := coursesRepository.CreateCourse(ctx, "publication-asset-binding")
	if err != nil {
		t.Fatal(err)
	}
	submitter := "ac000000-0000-4000-8000-000000000001"
	reviewer := "ac000000-0000-4000-8000-000000000002"
	publisher := "ac000000-0000-4000-8000-000000000003"
	metadata := draftFixture(t, course.ID)
	draft, _, err := authoringRepository.CreateDraft(ctx, metadata, submitter)
	if err != nil {
		t.Fatal(err)
	}
	storage, err := assetslocal.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ingestion, err := assets.NewIngestionService(assetRepository, storage, 1024)
	if err != nil {
		t.Fatal(err)
	}
	assetBytes := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	asset, err := ingestion.Ingest(ctx, assets.IngestionInput{
		OwnerDraftID: string(draft.ID), CreatedByUserID: submitter, OriginalFilename: "architecture.gif", Content: bytes.NewReader(assetBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	module, draft, err := authoringRepository.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "assets", Title: "Assets", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	lesson := lessonFixture(draft.ID, module.ID, "asset-lesson", 0)
	lesson.Content = courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{
		Key: "diagram", Type: courses.BlockImage,
		Payload: courses.ImageBlockPayload{Asset: courses.AssetReference{AssetKey: string(asset.ID)}, AltText: "Architecture"},
	}}}
	_, draft, err = authoringRepository.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, lesson)
	if err != nil {
		t.Fatal(err)
	}
	cycle, _, err := authoringRepository.SubmitReview(ctx, draft.ID, draft.Revision, submitter)
	if err != nil {
		t.Fatal(err)
	}
	cycle, err = authoringRepository.DecideReview(ctx, cycle.ID, cycle.Revision, authoring.ReviewApproved, reviewer, "")
	if err != nil {
		t.Fatal(err)
	}
	service := authoring.NewPublicationServiceWithAssets(authoringRepository, authoring.NewAssetPublicationResolver(assetRepository), courseStore, authoringRepository)
	command := authoring.PublishReviewCommand{DraftID: cycle.DraftID, ReviewID: cycle.ID, ExpectedReviewRevision: cycle.Revision, PublishedByUserID: publisher, PublishedAt: time.Date(2026, 9, 17, 15, 0, 0, 0, time.UTC)}
	result, err := service.Publish(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := courseStore.GetByReviewID(ctx, string(cycle.ID))
	if err != nil || stored.ID != result.CourseVersionID || len(stored.AssetBindings) != 1 {
		t.Fatalf("stored asset publication = %#v, %v", stored, err)
	}
	binding := stored.AssetBindings[0]
	if binding.AssetKey != string(asset.ID) || binding.StorageObjectID != string(asset.StorageObjectID) || binding.MediaType != asset.MediaType || binding.ByteSize != asset.ByteSize || binding.SHA256Digest != string(asset.SHA256Digest) || binding.OriginalFilename != asset.OriginalFilename {
		t.Fatalf("binding did not freeze authoritative metadata: %#v", binding)
	}
	image := stored.Modules[0].Lessons[0].Content.Blocks[0].Payload.(courses.ImageBlockPayload)
	if image.Asset.AssetKey != string(asset.ID) {
		t.Fatalf("canonical content was rewritten: %#v", image)
	}
	if _, err := pool.Exec(ctx, `UPDATE assets.asset SET original_filename='changed.png', media_type='text/plain' WHERE id=$1`, asset.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err := courseStore.GetByReviewID(ctx, string(cycle.ID))
	if err != nil || !reflect.DeepEqual(reloaded.AssetBindings, stored.AssetBindings) {
		t.Fatalf("mutable Authoring Asset altered publication: %#v, %v", reloaded.AssetBindings, err)
	}
	replayed, err := service.Publish(ctx, command)
	if err != nil || !replayed.Reconciled || replayed.CourseVersionID != stored.ID {
		t.Fatalf("replay did not retain frozen binding: %#v, %v", replayed, err)
	}
	var bindingCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM courses.course_version_asset_binding WHERE course_version_id=$1`, stored.ID).Scan(&bindingCount); err != nil || bindingCount != 1 {
		t.Fatalf("binding count = %d, %v", bindingCount, err)
	}

	router := authTestRouter(&authHTTP{publishedAssets: courses.NewPublishedAssetReadService(coursesRepository), assetStorage: storage})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/courses/by-id/"+string(course.ID)+"/versions/"+result.CourseVersion.String()+"/assets/"+string(asset.ID), nil))
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), assetBytes) || response.Header().Get("Content-Type") != "image/gif" {
		t.Fatalf("published asset delivery = status=%d headers=%#v body=%q", response.Code, response.Header(), response.Body.Bytes())
	}
}

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
	type publicationAttempt struct {
		result authoring.PublicationResult
		err    error
	}
	results := make(chan publicationAttempt, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-barrier
			result, publishErr := concurrentService.Publish(ctx, concurrentCommand)
			results <- publicationAttempt{result: result, err: publishErr}
		}()
	}
	close(barrier)
	wait.Wait()
	close(results)
	var concurrentVersionID courses.CourseVersionID
	for attempt := range results {
		if attempt.err != nil {
			t.Fatalf("concurrent publication did not converge: %v", attempt.err)
		}
		if attempt.result.ReviewID != concurrent.ID || attempt.result.ReviewRevision != concurrent.Revision || attempt.result.CourseID != course.ID || attempt.result.CourseVersion.String() != "2.0.0" || attempt.result.CourseVersionID == "" {
			t.Fatalf("concurrent publication result is incomplete: %#v", attempt.result)
		}
		if concurrentVersionID == "" {
			concurrentVersionID = attempt.result.CourseVersionID
		} else if attempt.result.CourseVersionID != concurrentVersionID {
			t.Fatalf("concurrent callers observed different CourseVersion IDs: %q and %q", concurrentVersionID, attempt.result.CourseVersionID)
		}
	}
	assertPublicationCounts(t, ctx, pool, course.ID, 2, 2)
	concurrentStored, err := courseStore.GetByReviewID(ctx, string(concurrent.ID))
	if err != nil || concurrentStored.ID != concurrentVersionID {
		t.Fatalf("concurrent CourseVersion readback = %#v %v", concurrentStored, err)
	}
	concurrentFact, err := authoringRepository.GetPublication(ctx, concurrent.ID)
	if err != nil || concurrentFact.CourseVersionID != concurrentVersionID || concurrentFact.ReviewRevision != concurrent.Revision || concurrentFact.CourseVersion.String() != "2.0.0" {
		t.Fatalf("concurrent publication fact = %#v %v", concurrentFact, err)
	}
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
	administrator, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := identityRepository.AssignGlobalRole(ctx, administrator.ID, identity.RoleAdministrator, nil); err != nil {
		t.Fatal(err)
	}
	revokedMaintainer, err := identityRepository.CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	otherDraftMaintainer, err := identityRepository.CreateUser(ctx, identity.UserActive)
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
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, workspace.ID, string(revokedMaintainer.ID), authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	if _, err := revokeTestAuthoringMember(ctx, pool, authoringRepository, workspace.ID, string(revokedMaintainer.ID)); err != nil {
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
	administratorSession, err := sessions.CreateSession(ctx, administrator.ID)
	if err != nil {
		t.Fatal(err)
	}
	revokedMaintainerSession, err := sessions.CreateSession(ctx, revokedMaintainer.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherDraftMaintainerSession, err := sessions.CreateSession(ctx, otherDraftMaintainer.ID)
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
	administratorCookie := &http.Cookie{Name: sessionCookieName, Value: administratorSession.Token.Value()}
	revokedMaintainerCookie := &http.Cookie{Name: sessionCookieName, Value: revokedMaintainerSession.Token.Value()}
	otherDraftMaintainerCookie := &http.Cookie{Name: sessionCookieName, Value: otherDraftMaintainerSession.Token.Value()}

	denied := authRequest(router, http.MethodPost, path, body, unauthorizedCookie, csrf)
	if denied.Code != http.StatusNotFound {
		t.Fatalf("AUTHOR publication = %d %s", denied.Code, denied.Body.String())
	}
	if denied := authRequest(router, http.MethodPost, path, body, administratorCookie, csrf); denied.Code != http.StatusNotFound {
		t.Fatalf("global ADMIN publication = %d %s", denied.Code, denied.Body.String())
	}
	if denied := authRequest(router, http.MethodPost, path, body, revokedMaintainerCookie, csrf); denied.Code != http.StatusNotFound {
		t.Fatalf("revoked MAINTAINER publication = %d %s", denied.Code, denied.Body.String())
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
	var recoveredPublication authoringPublicationDTO
	if err := json.Unmarshal(recovered.Body.Bytes(), &recoveredPublication); err != nil || recoveredPublication.CourseVersionID == "" {
		t.Fatalf("reconciled publication body = %s: %v", recovered.Body.String(), err)
	}
	exactReplay := authRequest(router, http.MethodPost, path, body, publisherCookie, csrf)
	if exactReplay.Code != http.StatusOK {
		t.Fatalf("exact replay = %d %s", exactReplay.Code, exactReplay.Body.String())
	}
	var replayPublication authoringPublicationDTO
	if err := json.Unmarshal(exactReplay.Body.Bytes(), &replayPublication); err != nil || replayPublication.CourseVersionID != recoveredPublication.CourseVersionID || !replayPublication.PublishedAt.Equal(recoveredPublication.PublishedAt) || replayPublication.CourseVersion != recoveredPublication.CourseVersion {
		t.Fatalf("exact replay did not preserve immutable publication result: %#v / %#v / %v", recoveredPublication, replayPublication, err)
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
	publicCourse, err := courses.NewPublishedReadService(coursesRepository).Exact(ctx, course.ID, stored.CourseVersion.Version)
	if err != nil || len(publicCourse.Contributors) != 1 || publicCourse.Contributors[0].DisplayName != "Author" {
		t.Fatalf("public publication attribution = %#v %v", publicCourse.Contributors, err)
	}
	publicJSON, err := json.Marshal(publishedCourseVersion(publicCourse))
	if err != nil {
		t.Fatal(err)
	}
	for _, internal := range []string{string(submitter.ID), reviewer, string(publisher.ID), string(cycle.ID), string(cycle.DraftID)} {
		if strings.Contains(string(publicJSON), internal) {
			t.Fatalf("public CourseVersion leaked internal publication provenance %q: %s", internal, publicJSON)
		}
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
	if _, err := addTestAuthoringMember(ctx, pool, authoringRepository, conflictingWorkspace.ID, string(otherDraftMaintainer.ID), authoring.MemberMaintainer); err != nil {
		t.Fatal(err)
	}
	if denied := authRequest(router, http.MethodPost, path, body, otherDraftMaintainerCookie, csrf); denied.Code != http.StatusNotFound || strings.Contains(denied.Body.String(), `"courseVersion"`) {
		t.Fatalf("other-Draft MAINTAINER publication = %d %s", denied.Code, denied.Body.String())
	}
	wrongDraftPath := "/api/authoring/drafts/" + string(conflicting.DraftID) + "/reviews/" + string(cycle.ID) + "/publish"
	wrongDraft := authRequest(router, http.MethodPost, wrongDraftPath, body, publisherCookie, csrf)
	if wrongDraft.Code != http.StatusNotFound || strings.Contains(wrongDraft.Body.String(), `"courseVersion"`) {
		t.Fatalf("cross-Draft Review publication = %d %s", wrongDraft.Code, wrongDraft.Body.String())
	}
	assertPublicationCounts(t, ctx, pool, course.ID, 1, 1)
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
