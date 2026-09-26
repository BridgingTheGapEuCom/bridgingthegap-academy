package assessments

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

const (
	testDraftID   = "10000000-0000-4000-8000-000000000001"
	testCreatorID = "20000000-0000-4000-8000-000000000001"
)

func validQuestions() []Question {
	return []Question{
		{StableKey: "single", Type: QuestionSingleChoice, Prompt: "Choose one.", Position: 0,
			Options: []ChoiceOption{{StableKey: "first", Text: "First", Position: 0}, {StableKey: "second", Text: "Second", Position: 1}}, CorrectOptionKeys: []string{"second"}},
		{StableKey: "multiple", Type: QuestionMultipleChoice, Prompt: "Choose all.", Position: 1,
			Options: []ChoiceOption{{StableKey: "one", Text: "One", Position: 0}, {StableKey: "two", Text: "Two", Position: 1}, {StableKey: "three", Text: "Three", Position: 2}}, CorrectOptionKeys: []string{"one", "three"}},
		{StableKey: "matching", Type: QuestionMatching, Prompt: "Match each item.", Position: 2,
			LeftItems:    []MatchingItem{{StableKey: "left-a", Text: "A", Position: 0}, {StableKey: "left-b", Text: "B", Position: 1}},
			RightItems:   []MatchingItem{{StableKey: "right-one", Text: "One", Position: 0}, {StableKey: "right-two", Text: "Two", Position: 1}},
			CorrectPairs: []MatchingPair{{LeftKey: "left-a", RightKey: "right-two"}, {LeftKey: "left-b", RightKey: "right-one"}}},
	}
}

func validInput() AssessmentInput {
	return AssessmentInput{OwnerDraftID: testDraftID, Title: "Core concepts", Questions: validQuestions(), CreatedByUserID: testCreatorID}
}

func TestAssessmentInputSupportsClosedDeterministicQuestionTypes(t *testing.T) {
	if err := validInput().Validate(); err != nil {
		t.Fatalf("valid deterministic questions rejected: %v", err)
	}
}

func TestAssessmentValidationRejectsInvalidAnswerDefinitions(t *testing.T) {
	tests := map[string]func(*AssessmentInput){
		"duplicate question key": func(input *AssessmentInput) { input.Questions[1].StableKey = input.Questions[0].StableKey },
		"duplicate option key": func(input *AssessmentInput) {
			input.Questions[0].Options[1].StableKey = input.Questions[0].Options[0].StableKey
		},
		"single choice has no answer":      func(input *AssessmentInput) { input.Questions[0].CorrectOptionKeys = nil },
		"single choice has two answers":    func(input *AssessmentInput) { input.Questions[0].CorrectOptionKeys = []string{"first", "second"} },
		"multiple choice has no answer":    func(input *AssessmentInput) { input.Questions[1].CorrectOptionKeys = nil },
		"multiple choice repeats answer":   func(input *AssessmentInput) { input.Questions[1].CorrectOptionKeys = []string{"one", "one"} },
		"answer references missing option": func(input *AssessmentInput) { input.Questions[1].CorrectOptionKeys = []string{"missing"} },
		"matching references missing item": func(input *AssessmentInput) { input.Questions[2].CorrectPairs[0].RightKey = "missing" },
		"matching repeats item":            func(input *AssessmentInput) { input.Questions[2].CorrectPairs[1].LeftKey = "left-a" },
		"unsupported type":                 func(input *AssessmentInput) { input.Questions[0].Type = "ESSAY" },
		"invalid draft":                    func(input *AssessmentInput) { input.OwnerDraftID = "invalid" },
		"invalid creator":                  func(input *AssessmentInput) { input.CreatedByUserID = "invalid" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			input := validInput()
			input.Questions = append([]Question(nil), input.Questions...)
			for index := range input.Questions {
				input.Questions[index].Options = append([]ChoiceOption(nil), input.Questions[index].Options...)
				input.Questions[index].CorrectOptionKeys = append([]string(nil), input.Questions[index].CorrectOptionKeys...)
				input.Questions[index].CorrectPairs = append([]MatchingPair(nil), input.Questions[index].CorrectPairs...)
			}
			mutate(&input)
			if !errors.Is(input.Validate(), ErrInvalidAssessment) {
				t.Fatalf("invalid assessment accepted: %#v", input)
			}
		})
	}
}

func TestAssessmentAllowsEmptyMutableDefinitionButKeepsOrderSeparateFromKeys(t *testing.T) {
	empty := validInput()
	empty.Questions = nil
	if err := empty.Validate(); err != nil {
		t.Fatalf("empty mutable assessment rejected: %v", err)
	}
	input := validInput()
	input.Questions[0], input.Questions[1] = input.Questions[1], input.Questions[0]
	input.Questions[0].Position, input.Questions[1].Position = 0, 1
	if err := input.Validate(); err != nil {
		t.Fatalf("reordering changed stable identity validity: %v", err)
	}
	if input.Questions[0].StableKey != "multiple" || input.Questions[1].StableKey != "single" {
		t.Fatal("question identity was coupled to order")
	}
}

func TestAssessmentJSONDoesNotExposeQuestionsAnswersOrCreator(t *testing.T) {
	assessment := Assessment{
		ID: "30000000-0000-4000-8000-000000000001", OwnerDraftID: testDraftID, Title: "Core concepts", Revision: 1,
		Questions: validQuestions(), CreatedByUserID: testCreatorID,
		CreatedAt: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC),
	}
	if err := assessment.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(assessment)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{testDraftID, testCreatorID, "correctOptionKeys", "left-a"} {
		if string(encoded) != "" && contains(string(encoded), private) {
			t.Fatalf("private assessment data leaked from default JSON: %s", encoded)
		}
	}
}

func TestAssessmentValidationIsDeterministic(t *testing.T) {
	left, right := validInput(), validInput()
	left.Questions[0].CorrectOptionKeys = []string{"missing"}
	right.Questions[0].CorrectOptionKeys = []string{"missing"}
	if !reflect.DeepEqual(left.Validate(), right.Validate()) || !errors.Is(left.Validate(), ErrInvalidAssessment) {
		t.Fatalf("validation was not deterministic: %v / %v", left.Validate(), right.Validate())
	}
}

func contains(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}
