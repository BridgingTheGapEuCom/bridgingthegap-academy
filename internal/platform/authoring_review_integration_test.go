//go:build integration

package platform

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringReviewPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepository := coursespostgres.New(pool)
	course, err := courseRepository.CreateCourse(ctx, "authoring-review-persistence")
	if err != nil {
		t.Fatal(err)
	}
	repository := authoringpostgres.New(pool)
	submitter := "a1000000-0000-4000-8000-000000000001"
	reviewerA := "a1000000-0000-4000-8000-000000000002"
	reviewerB := "a1000000-0000-4000-8000-000000000003"
	draft, _, err := repository.CreateDraft(ctx, draftFixture(t, course.ID), submitter)
	if err != nil {
		t.Fatal(err)
	}

	module, draft, err := repository.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{
		DraftID: draft.ID, StableKey: "review-foundations", Title: "Review foundations", Position: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	content := courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "boundary", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}}
	firstInput := lessonFixture(draft.ID, module.ID, "review-introduction", 0)
	firstInput.Content = content
	first, draft, err := repository.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, firstInput)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := lessonFixture(draft.ID, module.ID, "review-practice", 1)
	second, draft, err := repository.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	_, draft, err = repository.ReplaceLessonPrerequisitesForDraft(ctx, draft.ID, second.ID, second.Revision, []string{first.StableKey})
	if err != nil {
		t.Fatal(err)
	}

	cycle, snapshot, err := repository.SubmitReview(ctx, draft.ID, draft.Revision, submitter)
	if err != nil {
		t.Fatal(err)
	}
	if cycle.Status != authoring.ReviewInReview || cycle.DraftRevision != draft.Revision || snapshot.Draft.Revision != draft.Revision {
		t.Fatalf("submission did not bind exact revision: %#v %#v", cycle, snapshot.Draft)
	}
	if len(snapshot.Modules) != 1 || len(snapshot.Modules[0].Lessons) != 2 || snapshot.Modules[0].Lessons[1].PrerequisiteStableKeys[0] != first.StableKey || snapshot.Modules[0].Lessons[0].Content.Blocks[0].Key != "boundary" {
		t.Fatalf("canonical snapshot incomplete: %#v", snapshot.Modules)
	}
	active, _, err := repository.ActiveReview(ctx, draft.ID)
	if err != nil || active.ID != cycle.ID {
		t.Fatalf("active review lookup: %v %#v", err, active)
	}
	if _, _, err := repository.SubmitReview(ctx, draft.ID, draft.Revision, submitter); !errors.Is(err, authoring.ErrReviewAlreadyExists) {
		t.Fatalf("duplicate revision submission: %v", err)
	}

	// Later Draft edits cannot alter the immutable frozen snapshot.
	changed := draft.Metadata
	changed.Title = "Changed after submission"
	draft, err = repository.UpdateDraftMetadata(ctx, draft.ID, draft.Revision, changed)
	if err != nil {
		t.Fatal(err)
	}
	_, frozen, err := repository.GetReview(ctx, cycle.ID)
	if err != nil || frozen.Draft.Title == changed.Title || frozen.Draft.Revision == draft.Revision {
		t.Fatalf("later edit mutated frozen review: %v %#v", err, frozen.Draft)
	}

	// Mutually exclusive reviewer decisions share a Review revision CAS.
	barrier := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index, decision := range []authoring.ReviewStatus{authoring.ReviewApproved, authoring.ReviewChangesRequested} {
		index, decision := index, decision
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-barrier
			_, decideErr := repository.DecideReview(ctx, cycle.ID, cycle.Revision, decision, []string{reviewerA, reviewerB}[index], "Decision")
			results <- decideErr
		}()
	}
	close(barrier)
	wait.Wait()
	close(results)
	succeeded, rejected := 0, 0
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, authoring.ErrReviewInvalidState), errors.Is(result, authoring.ErrReviewStale):
			rejected++
		default:
			t.Fatalf("unexpected decision error: %v", result)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("conflicting decisions: success=%d rejected=%d", succeeded, rejected)
	}
	decided, decidedSnapshot, err := repository.GetReview(ctx, cycle.ID)
	if err != nil || decided.Revision != 2 || decided.DraftRevision != cycle.DraftRevision || decidedSnapshot.Draft.Title != snapshot.Draft.Title {
		t.Fatalf("decision rewrote provenance: %v %#v", err, decided)
	}
	events, err := repository.ListReviewEvents(ctx, cycle.ID)
	if err != nil || len(events) != 2 || events[0].Type != authoring.ReviewSubmittedEvent {
		t.Fatalf("review event history: %v %#v", err, events)
	}
	if decided.Status == authoring.ReviewApproved {
		approved, approvedSnapshot, lookupErr := repository.ApprovedReviewForRevision(ctx, draft.ID, cycle.DraftRevision)
		if lookupErr != nil || approved.ID != cycle.ID || approvedSnapshot.Draft.Revision != cycle.DraftRevision {
			t.Fatalf("exact approved lookup: %v", lookupErr)
		}
	}

	// Database constraints keep frozen provenance and event history append-only.
	if _, err := pool.Exec(ctx, "UPDATE authoring.review_cycle SET draft_revision = draft_revision + 1 WHERE id = $1", cycle.ID); err == nil {
		t.Fatal("database allowed frozen revision rewrite")
	}
	if _, err := pool.Exec(ctx, "DELETE FROM authoring.review_event WHERE review_id = $1", cycle.ID); err == nil {
		t.Fatal("database allowed review history deletion")
	}

	history, err := repository.ListReviewHistory(ctx, draft.ID)
	if err != nil || len(history) != 1 || history[0].ID != cycle.ID {
		t.Fatalf("review history ordering: %v %#v", err, history)
	}

	// Two submissions for the same exact Draft revision cannot both create a cycle.
	concurrentDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, course.ID), submitter)
	if err != nil {
		t.Fatal(err)
	}
	submitBarrier := make(chan struct{})
	submitResults := make(chan error, 2)
	wait = sync.WaitGroup{}
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-submitBarrier
			_, _, submitErr := repository.SubmitReview(ctx, concurrentDraft.ID, concurrentDraft.Revision, submitter)
			submitResults <- submitErr
		}()
	}
	close(submitBarrier)
	wait.Wait()
	close(submitResults)
	succeeded, rejected = 0, 0
	for result := range submitResults {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, authoring.ErrReviewAlreadyExists):
			rejected++
		default:
			t.Fatalf("unexpected concurrent submission error: %v", result)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent submissions: success=%d rejected=%d", succeeded, rejected)
	}

	// Requested changes close one immutable cycle; a later revision creates a
	// new cycle rather than retargeting the old snapshot.
	resubmitDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, course.ID), submitter)
	if err != nil {
		t.Fatal(err)
	}
	firstCycle, firstSnapshot, err := repository.SubmitReview(ctx, resubmitDraft.ID, resubmitDraft.Revision, submitter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.DecideReview(ctx, firstCycle.ID, firstCycle.Revision, authoring.ReviewChangesRequested, reviewerA, "Clarify the title."); err != nil {
		t.Fatal(err)
	}
	resubmitMetadata := resubmitDraft.Metadata
	resubmitMetadata.Title = "Revised after feedback"
	resubmitDraft, err = repository.UpdateDraftMetadata(ctx, resubmitDraft.ID, resubmitDraft.Revision, resubmitMetadata)
	if err != nil {
		t.Fatal(err)
	}
	secondCycle, secondSnapshot, err := repository.SubmitReview(ctx, resubmitDraft.ID, resubmitDraft.Revision, submitter)
	if err != nil {
		t.Fatal(err)
	}
	if secondCycle.ID == firstCycle.ID || secondCycle.DraftRevision == firstCycle.DraftRevision || firstSnapshot.Draft.Title == secondSnapshot.Draft.Title {
		t.Fatal("resubmission rewrote an earlier review cycle")
	}
	resubmitHistory, err := repository.ListReviewHistory(ctx, resubmitDraft.ID)
	if err != nil || len(resubmitHistory) != 2 || resubmitHistory[0].ID != secondCycle.ID {
		t.Fatalf("resubmission history: %v %#v", err, resubmitHistory)
	}

	// A Draft edit racing submission either wins first and makes submission
	// stale, or follows a snapshot that still contains exactly the old state.
	raceDraft, _, err := repository.CreateDraft(ctx, draftFixture(t, course.ID), submitter)
	if err != nil {
		t.Fatal(err)
	}
	raceMetadata := raceDraft.Metadata
	raceMetadata.Title = "Concurrent edit"
	raceBarrier := make(chan struct{})
	var raceSnapshot authoring.ReviewSnapshot
	var submitErr, editErr error
	wait = sync.WaitGroup{}
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-raceBarrier
		_, raceSnapshot, submitErr = repository.SubmitReview(ctx, raceDraft.ID, raceDraft.Revision, submitter)
	}()
	go func() {
		defer wait.Done()
		<-raceBarrier
		_, editErr = repository.UpdateDraftMetadata(ctx, raceDraft.ID, raceDraft.Revision, raceMetadata)
	}()
	close(raceBarrier)
	wait.Wait()
	if editErr != nil {
		t.Fatalf("racing Draft edit failed unexpectedly: %v", editErr)
	}
	if submitErr == nil {
		if raceSnapshot.Draft.Title != raceDraft.Metadata.Title || raceSnapshot.Draft.Revision != raceDraft.Revision {
			t.Fatalf("submission captured a mixed Draft state: %#v", raceSnapshot.Draft)
		}
	} else if !errors.Is(submitErr, authoring.ErrRevisionMismatch) {
		t.Fatalf("unexpected racing submission result: %v", submitErr)
	}
}
