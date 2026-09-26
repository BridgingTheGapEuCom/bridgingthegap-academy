package authoring

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

func frozenAssessmentFixture(key string) ReviewSnapshotAssessment {
	return ReviewSnapshotAssessment{AssessmentKey: key, Questions: []assessments.Question{
		{StableKey: "single", Type: assessments.QuestionSingleChoice, Prompt: "Choose one", Position: 0, Options: []assessments.ChoiceOption{{StableKey: "one", Text: "One", Position: 0}, {StableKey: "two", Text: "Two", Position: 1}}, CorrectOptionKeys: []string{"one"}},
		{StableKey: "multiple", Type: assessments.QuestionMultipleChoice, Prompt: "Choose several", Position: 1, Options: []assessments.ChoiceOption{{StableKey: "one", Text: "One", Position: 0}, {StableKey: "two", Text: "Two", Position: 1}}, CorrectOptionKeys: []string{"one", "two"}},
		{StableKey: "matching", Type: assessments.QuestionMatching, Prompt: "Match", Position: 2, LeftItems: []assessments.MatchingItem{{StableKey: "left-one", Text: "Left one", Position: 0}, {StableKey: "left-two", Text: "Left two", Position: 1}}, RightItems: []assessments.MatchingItem{{StableKey: "right-one", Text: "Right one", Position: 0}, {StableKey: "right-two", Text: "Right two", Position: 1}}, CorrectPairs: []assessments.MatchingPair{{LeftKey: "left-one", RightKey: "right-two"}, {LeftKey: "left-two", RightKey: "right-one"}}},
	}}
}

func snapshotWithAssessmentReferences(t *testing.T, keys ...string) (ReviewCycle, ReviewSnapshot) {
	t.Helper()
	cycle, snapshot := publicationValidationFixture(t)
	blocks := make([]courses.Block, 0, len(keys))
	for index, key := range keys {
		blocks = append(blocks, courses.Block{Key: "check-" + string(rune('a'+index)), Type: courses.BlockKnowledgeCheck, Payload: courses.KnowledgeCheckBlockPayload{AssessmentKey: key}})
	}
	snapshot.Modules[0].Lessons[0].Content = courses.LessonContent{SchemaVersion: 1, Blocks: blocks}
	return cycle, snapshot
}

func TestPublicationAssessmentResolutionFreezesAllSupportedDefinitions(t *testing.T) {
	key := "71000000-0000-4000-8000-000000000001"
	cycle, snapshot := snapshotWithAssessmentReferences(t, key, key)
	snapshot.Assessments = []ReviewSnapshotAssessment{frozenAssessmentFixture(key)}
	resolution := resolvePublicationAssessments(snapshot)
	if len(resolution.Issues) != 0 || len(resolution.Bindings) != 1 {
		t.Fatalf("resolution = %#v", resolution)
	}
	binding := resolution.Bindings[0]
	if len(binding.Questions) != 3 || binding.Questions[0].CorrectOptionKeys[0] != "one" || binding.Questions[2].CorrectPairs[0].RightKey != "right-two" {
		t.Fatalf("frozen binding lost semantics: %#v", binding)
	}
	if !NewPublicationValidator().ValidateResolvedBindings(cycle, &snapshot, nil, resolution.Bindings).Publishable {
		t.Fatal("valid frozen Assessment did not resolve publication")
	}
	if reflect.ValueOf(binding).FieldByName("OwnerDraftID").IsValid() || reflect.ValueOf(binding).FieldByName("CreatedByUserID").IsValid() {
		t.Fatal("Authoring provenance entered Courses binding")
	}
}

func TestPublicationAssessmentResolutionBlocksUnavailableAndIncompleteSafely(t *testing.T) {
	missing := "72000000-0000-4000-8000-000000000001"
	empty := "72000000-0000-4000-8000-000000000002"
	_, snapshot := snapshotWithAssessmentReferences(t, "legacy", missing, empty)
	snapshot.Assessments = []ReviewSnapshotAssessment{{AssessmentKey: empty, Questions: []assessments.Question{}}}
	resolution := resolvePublicationAssessments(snapshot)
	want := []PublicationValidationCode{PublicationIssueAssessmentUnavailable, PublicationIssueAssessmentUnavailable, PublicationIssueAssessmentIncomplete}
	got := make([]PublicationValidationCode, 0, len(resolution.Issues))
	for _, issue := range resolution.Issues {
		got = append(got, issue.Code)
		if issue.Path == "" || issue.Message == "" {
			t.Fatalf("unsafe issue: %#v", issue)
		}
	}
	if !reflect.DeepEqual(got, want) || len(resolution.Bindings) != 0 {
		t.Fatalf("resolution = %#v", resolution)
	}
}

func TestFrozenAssessmentAnswersPersistButStayOutOfReviewJSON(t *testing.T) {
	key := "73000000-0000-4000-8000-000000000001"
	_, snapshot := snapshotWithAssessmentReferences(t, key)
	snapshot.Assessments = []ReviewSnapshotAssessment{frozenAssessmentFixture(key)}
	persisted, err := MarshalReviewSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := UnmarshalReviewSnapshot(persisted)
	if err != nil || !reflect.DeepEqual(restored.Assessments, snapshot.Assessments) {
		t.Fatalf("private snapshot round-trip = %#v, %v", restored.Assessments, err)
	}
	public, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if string(public) == string(persisted) || jsonContainsKey(public, "assessments") || jsonContainsKey(public, "correctOptionKeys") {
		t.Fatalf("answer-bearing snapshot leaked through public JSON: %s", public)
	}
}

func jsonContainsKey(value []byte, key string) bool {
	var decoded any
	_ = json.Unmarshal(value, &decoded)
	return containsJSONKey(decoded, key)
}

func containsJSONKey(value any, key string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for candidate, child := range typed {
			if candidate == key || containsJSONKey(child, key) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsJSONKey(child, key) {
				return true
			}
		}
	}
	return false
}
