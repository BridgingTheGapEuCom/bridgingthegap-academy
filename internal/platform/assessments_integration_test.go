//go:build integration

package platform

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	assessmentspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAssessmentsPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	repository := assessmentspostgres.New(pool)
	input := assessments.AssessmentInput{
		OwnerDraftID:    "10000000-0000-4000-8000-000000000101",
		CreatedByUserID: "20000000-0000-4000-8000-000000000101",
		Title:           "Assessment persistence",
		Questions: []assessments.Question{
			{
				StableKey: "single", Type: assessments.QuestionSingleChoice, Prompt: "Pick one.", Position: 0,
				Options:           []assessments.ChoiceOption{{StableKey: "a", Text: "A", Position: 0}, {StableKey: "b", Text: "B", Position: 1}},
				CorrectOptionKeys: []string{"b"},
			},
			{
				StableKey: "multiple", Type: assessments.QuestionMultipleChoice, Prompt: "Pick all.", Position: 1,
				Options:           []assessments.ChoiceOption{{StableKey: "a", Text: "A", Position: 0}, {StableKey: "b", Text: "B", Position: 1}, {StableKey: "c", Text: "C", Position: 2}},
				CorrectOptionKeys: []string{"a", "c"},
			},
			{
				StableKey: "matching", Type: assessments.QuestionMatching, Prompt: "Match.", Position: 2,
				LeftItems:    []assessments.MatchingItem{{StableKey: "left-a", Text: "A", Position: 0}, {StableKey: "left-b", Text: "B", Position: 1}},
				RightItems:   []assessments.MatchingItem{{StableKey: "right-one", Text: "One", Position: 0}, {StableKey: "right-two", Text: "Two", Position: 1}},
				CorrectPairs: []assessments.MatchingPair{{LeftKey: "left-a", RightKey: "right-two"}, {LeftKey: "left-b", RightKey: "right-one"}},
			},
		},
	}
	created, err := repository.CreateAssessment(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Revision != 1 || created.OwnerDraftID != input.OwnerDraftID || created.CreatedByUserID != input.CreatedByUserID {
		t.Fatalf("created Assessment = %#v", created)
	}
	loaded, err := repository.GetAssessment(ctx, created.ID)
	if err != nil || !sameAssessment(created, loaded) {
		t.Fatalf("Assessment round trip = %#v, %v", loaded, err)
	}
	if !reflect.DeepEqual(loaded.Questions, input.Questions) {
		t.Fatalf("question definitions did not round trip: %#v", loaded.Questions)
	}
	var beforeInvalid int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM assessments.assessment").Scan(&beforeInvalid); err != nil {
		t.Fatal(err)
	}
	invalid := input
	invalid.Questions = append([]assessments.Question(nil), input.Questions...)
	invalid.Questions[0].CorrectOptionKeys = []string{"missing"}
	if _, err := repository.CreateAssessment(ctx, invalid); !errors.Is(err, assessments.ErrInvalidAssessment) {
		t.Fatalf("invalid aggregate create = %v", err)
	}
	var afterInvalid int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM assessments.assessment").Scan(&afterInvalid); err != nil || afterInvalid != beforeInvalid {
		t.Fatalf("invalid aggregate left persisted data count=%d err=%v", afterInvalid, err)
	}

	updated, err := repository.UpdateAssessment(ctx, created.ID, created.Revision, assessments.AssessmentUpdate{
		Title: "Updated assessment", Questions: input.Questions,
	})
	if err != nil || updated.Revision != 2 || updated.Title != "Updated assessment" {
		t.Fatalf("Assessment update = %#v, %v", updated, err)
	}
	if _, err := repository.UpdateAssessment(ctx, created.ID, created.Revision, assessments.AssessmentUpdate{Title: "stale", Questions: input.Questions}); !errors.Is(err, assessments.ErrRevisionMismatch) {
		t.Fatalf("stale update = %v, want revision mismatch", err)
	}

	t.Run("concurrent updates have one winner", func(t *testing.T) {
		start := make(chan struct{})
		results := make(chan error, 2)
		var wait sync.WaitGroup
		for _, title := range []string{"concurrent one", "concurrent two"} {
			wait.Add(1)
			go func(title string) {
				defer wait.Done()
				<-start
				_, err := repository.UpdateAssessment(ctx, created.ID, updated.Revision, assessments.AssessmentUpdate{Title: title, Questions: input.Questions})
				results <- err
			}(title)
		}
		close(start)
		wait.Wait()
		close(results)
		succeeded, stale := 0, 0
		for err := range results {
			if err == nil {
				succeeded++
			} else if errors.Is(err, assessments.ErrRevisionMismatch) {
				stale++
			} else {
				t.Fatalf("concurrent update = %v", err)
			}
		}
		if succeeded != 1 || stale != 1 {
			t.Fatalf("concurrent update results success=%d stale=%d", succeeded, stale)
		}
	})

	var definitions int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM assessments.assessment WHERE id = $1", created.ID).Scan(&definitions); err != nil || definitions != 1 {
		t.Fatalf("aggregate row count = %d, %v", definitions, err)
	}
}

func sameAssessment(left, right assessments.Assessment) bool {
	return left.ID == right.ID && left.OwnerDraftID == right.OwnerDraftID && left.Title == right.Title && left.Revision == right.Revision &&
		left.CreatedByUserID == right.CreatedByUserID && left.CreatedAt.Equal(right.CreatedAt) && left.UpdatedAt.Equal(right.UpdatedAt) &&
		reflect.DeepEqual(left.Questions, right.Questions)
}
