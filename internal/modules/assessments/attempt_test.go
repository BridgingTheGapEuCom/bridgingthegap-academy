package assessments

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

const (
	testAttemptID       = "30000000-0000-4000-8000-000000000001"
	testLearnerID       = "40000000-0000-4000-8000-000000000001"
	testCourseVersionID = "50000000-0000-4000-8000-000000000001"
	testAssessmentID    = "60000000-0000-4000-8000-000000000001"
)

var testAttemptTime = time.Date(2026, time.September, 22, 10, 0, 0, 0, time.UTC)

func validAttemptResponses() []AttemptResponse {
	return []AttemptResponse{
		{QuestionKey: "matching", Type: QuestionMatching, Pairs: []AttemptMatchingPair{{LeftItemKey: "left-a", RightItemKey: "right-one"}, {LeftItemKey: "left-b", RightItemKey: "right-two"}}},
		{QuestionKey: "multiple", Type: QuestionMultipleChoice, SelectedOptionKeys: []string{"one", "two"}},
		{QuestionKey: "single", Type: QuestionSingleChoice, SelectedOptionKey: "one"},
	}
}

func validAttempt() AssessmentAttempt {
	return AssessmentAttempt{
		ID: testAttemptID, LearnerUserID: testLearnerID, CourseVersionID: testCourseVersionID, AssessmentKey: testAssessmentID,
		State: AttemptInProgress, Revision: 1, Responses: validAttemptResponses(), CreatedAt: testAttemptTime, UpdatedAt: testAttemptTime,
	}
}

func TestAssessmentAttemptSupportsPartialAndClosedResponses(t *testing.T) {
	partial := AttemptInput{LearnerUserID: testLearnerID, CourseVersionID: testCourseVersionID, AssessmentKey: testAssessmentID}
	if err := partial.Validate(); err != nil {
		t.Fatalf("empty in-progress response set rejected: %v", err)
	}
	emptyAttempt := validAttempt()
	emptyAttempt.Responses = nil
	if err := emptyAttempt.Validate(); err != nil {
		t.Fatalf("empty in-progress Attempt rejected: %v", err)
	}
	if err := validAttempt().Validate(); err != nil {
		t.Fatalf("valid deterministic responses rejected: %v", err)
	}
}

func TestAssessmentAttemptCanonicalizesSemanticResponseOrdering(t *testing.T) {
	responses := validAttemptResponses()
	responses[0], responses[2] = responses[2], responses[0]
	responses[1].SelectedOptionKeys = []string{"two", "one"}
	responses[2].Pairs = []AttemptMatchingPair{{LeftItemKey: "left-b", RightItemKey: "right-two"}, {LeftItemKey: "left-a", RightItemKey: "right-one"}}
	canonical, err := CanonicalAttemptResponses(responses)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{canonical[0].QuestionKey, canonical[1].QuestionKey, canonical[2].QuestionKey}; !reflect.DeepEqual(got, []string{"matching", "multiple", "single"}) {
		t.Fatalf("question response order = %#v", got)
	}
	if !reflect.DeepEqual(canonical[1].SelectedOptionKeys, []string{"one", "two"}) || canonical[0].Pairs[0].LeftItemKey != "left-a" {
		t.Fatalf("response values were not canonicalized: %#v", canonical)
	}
}

func TestAssessmentAttemptRejectsMalformedOrDuplicateResponses(t *testing.T) {
	tests := map[string]func(*AssessmentAttempt){
		"duplicate question":           func(attempt *AssessmentAttempt) { attempt.Responses[1].QuestionKey = attempt.Responses[0].QuestionKey },
		"duplicate multiple selection": func(attempt *AssessmentAttempt) { attempt.Responses[1].SelectedOptionKeys = []string{"one", "one"} },
		"duplicate matching left":      func(attempt *AssessmentAttempt) { attempt.Responses[0].Pairs[1].LeftItemKey = "left-a" },
		"invalid response type":        func(attempt *AssessmentAttempt) { attempt.Responses[0].Type = "ESSAY" },
		"invalid attempt id":           func(attempt *AssessmentAttempt) { attempt.ID = "invalid" },
		"invalid assessment key":       func(attempt *AssessmentAttempt) { attempt.AssessmentKey = "invalid" },
		"invalid state":                func(attempt *AssessmentAttempt) { attempt.State = "GRADED" },
		"in progress submitted at":     func(attempt *AssessmentAttempt) { at := testAttemptTime; attempt.SubmittedAt = &at },
		"submitted missing timestamp":  func(attempt *AssessmentAttempt) { attempt.State = AttemptSubmitted },
		"submitted missing result": func(attempt *AssessmentAttempt) {
			at := testAttemptTime.Add(time.Minute)
			attempt.State, attempt.SubmittedAt = AttemptSubmitted, &at
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			attempt := validAttempt()
			attempt.Responses = append([]AttemptResponse(nil), attempt.Responses...)
			for index := range attempt.Responses {
				attempt.Responses[index].SelectedOptionKeys = append([]string(nil), attempt.Responses[index].SelectedOptionKeys...)
				attempt.Responses[index].Pairs = append([]AttemptMatchingPair(nil), attempt.Responses[index].Pairs...)
			}
			mutate(&attempt)
			if !errors.Is(attempt.Validate(), ErrInvalidAttempt) {
				t.Fatalf("invalid attempt accepted: %#v", attempt)
			}
		})
	}
}

func TestAssessmentAttemptSubmissionIsOneWayAndPreservesResponses(t *testing.T) {
	attempt := validAttempt()
	submitted, err := attempt.Submit(AttemptResult{CorrectCount: 2, TotalCount: 3}, testAttemptTime.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if submitted.State != AttemptSubmitted || submitted.SubmittedAt == nil || submitted.Result == nil || *submitted.Result != (AttemptResult{CorrectCount: 2, TotalCount: 3}) || submitted.Revision != 2 || !reflect.DeepEqual(submitted.Responses, attempt.Responses) {
		t.Fatalf("submission did not preserve immutable attempt state: %#v", submitted)
	}
	if _, err := submitted.WithResponses(nil, testAttemptTime.Add(2*time.Minute)); !errors.Is(err, ErrAttemptImmutable) {
		t.Fatalf("submitted response update = %v, want immutable", err)
	}
	if _, err := submitted.Submit(AttemptResult{CorrectCount: 2, TotalCount: 3}, testAttemptTime.Add(2*time.Minute)); !errors.Is(err, ErrAttemptImmutable) {
		t.Fatalf("resubmit = %v, want immutable", err)
	}
}
