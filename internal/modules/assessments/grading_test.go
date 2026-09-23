package assessments

import (
	"reflect"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

func gradingBinding() courses.PublishedAssessmentBinding {
	return courses.PublishedAssessmentBinding{AssessmentKey: testAssessmentID, Questions: []courses.PublishedAssessmentQuestion{
		{StableKey: "single", Type: courses.PublishedQuestionSingleChoice, Prompt: "Choose one", Position: 0, Options: []courses.PublishedAssessmentOption{{StableKey: "one", Text: "One", Position: 0}, {StableKey: "two", Text: "Two", Position: 1}}, CorrectOptionKeys: []string{"one"}},
		{StableKey: "multiple", Type: courses.PublishedQuestionMultipleChoice, Prompt: "Choose all", Position: 1, Options: []courses.PublishedAssessmentOption{{StableKey: "one", Text: "One", Position: 0}, {StableKey: "two", Text: "Two", Position: 1}, {StableKey: "three", Text: "Three", Position: 2}}, CorrectOptionKeys: []string{"one", "two"}},
		{StableKey: "matching", Type: courses.PublishedQuestionMatching, Prompt: "Match", Position: 2, LeftItems: []courses.PublishedAssessmentItem{{StableKey: "left-a", Text: "A", Position: 0}, {StableKey: "left-b", Text: "B", Position: 1}}, RightItems: []courses.PublishedAssessmentItem{{StableKey: "right-one", Text: "One", Position: 0}, {StableKey: "right-two", Text: "Two", Position: 1}}, CorrectPairs: []courses.PublishedAssessmentPair{{LeftKey: "left-a", RightKey: "right-one"}, {LeftKey: "left-b", RightKey: "right-two"}}},
	}}
}

func completeCorrectResponses() []AttemptResponse {
	return []AttemptResponse{
		{QuestionKey: "single", Type: QuestionSingleChoice, SelectedOptionKey: "one"},
		{QuestionKey: "multiple", Type: QuestionMultipleChoice, SelectedOptionKeys: []string{"two", "one"}},
		{QuestionKey: "matching", Type: QuestionMatching, Pairs: []AttemptMatchingPair{{LeftItemKey: "left-b", RightItemKey: "right-two"}, {LeftItemKey: "left-a", RightItemKey: "right-one"}}},
	}
}

func TestGradePublishedAssessmentUsesExactSetAndMappingSemantics(t *testing.T) {
	result, err := GradePublishedAssessment(gradingBinding(), completeCorrectResponses())
	if err != nil || result != (AttemptResult{CorrectCount: 3, TotalCount: 3}) {
		t.Fatalf("all correct = %#v, %v", result, err)
	}
	responses := completeCorrectResponses()
	responses[0].SelectedOptionKey = "two"
	responses[1].SelectedOptionKeys = []string{"one"}
	responses[2].Pairs[0].RightItemKey = "right-one"
	responses[2].Pairs[1].RightItemKey = "right-two"
	result, err = GradePublishedAssessment(gradingBinding(), responses)
	if err != nil || result != (AttemptResult{CorrectCount: 0, TotalCount: 3}) {
		t.Fatalf("wrong but valid answers = %#v, %v", result, err)
	}
}

func TestValidateResponsesForPublishedAssessmentAllowsPartialButRejectsForeignSemantics(t *testing.T) {
	partial := []AttemptResponse{{QuestionKey: "single", Type: QuestionSingleChoice, SelectedOptionKey: "one"}}
	got, err := ValidateResponsesForPublishedAssessment(gradingBinding(), partial, false)
	if err != nil || !reflect.DeepEqual(got, partial) {
		t.Fatalf("partial response = %#v, %v", got, err)
	}
	if _, err := ValidateResponsesForPublishedAssessment(gradingBinding(), partial, true); err == nil {
		t.Fatal("incomplete submission accepted")
	}
	for name, response := range map[string][]AttemptResponse{
		"unknown question":  {{QuestionKey: "unknown", Type: QuestionSingleChoice, SelectedOptionKey: "one"}},
		"wrong variant":     {{QuestionKey: "single", Type: QuestionMultipleChoice, SelectedOptionKeys: []string{"one"}}},
		"unknown option":    {{QuestionKey: "single", Type: QuestionSingleChoice, SelectedOptionKey: "unknown"}},
		"duplicate choice":  {{QuestionKey: "multiple", Type: QuestionMultipleChoice, SelectedOptionKeys: []string{"one", "one"}}},
		"duplicate mapping": {{QuestionKey: "matching", Type: QuestionMatching, Pairs: []AttemptMatchingPair{{LeftItemKey: "left-a", RightItemKey: "right-one"}, {LeftItemKey: "left-a", RightItemKey: "right-two"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateResponsesForPublishedAssessment(gradingBinding(), response, false); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}
