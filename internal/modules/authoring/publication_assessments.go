package authoring

import (
	"fmt"
	"sort"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

const (
	PublicationIssueAssessmentUnavailable PublicationValidationCode = "unavailable_assessment_reference"
	PublicationIssueAssessmentIncomplete  PublicationValidationCode = "incomplete_assessment"
)

type PublicationAssessmentResolution struct {
	Bindings []courses.PublishedAssessmentBinding
	Issues   []PublicationValidationIssue
}

type publicationAssessmentUse struct{ key, path string }

// resolvePublicationAssessments maps only private definitions already frozen
// into the Review. It never reads the current mutable Assessment repository.
func resolvePublicationAssessments(snapshot ReviewSnapshot) PublicationAssessmentResolution {
	result := PublicationAssessmentResolution{Bindings: []courses.PublishedAssessmentBinding{}, Issues: []PublicationValidationIssue{}}
	frozen := make(map[string]ReviewSnapshotAssessment, len(snapshot.Assessments))
	for _, assessment := range snapshot.Assessments {
		frozen[assessment.AssessmentKey] = assessment
	}
	resolved := make(map[string]struct{})
	for _, use := range publicationAssessmentUses(snapshot) {
		if _, err := assessments.ParseAssessmentID(use.key); err != nil {
			result.Issues = append(result.Issues, unavailableAssessmentIssue(use.path))
			continue
		}
		assessment, exists := frozen[use.key]
		if !exists {
			result.Issues = append(result.Issues, unavailableAssessmentIssue(use.path))
			continue
		}
		if len(assessment.Questions) == 0 {
			result.Issues = append(result.Issues, PublicationValidationIssue{Code: PublicationIssueAssessmentIncomplete, Path: use.path, Message: "This Assessment must contain at least one complete question before publication."})
			continue
		}
		if _, duplicate := resolved[use.key]; duplicate {
			continue
		}
		binding, err := publishedAssessmentBinding(assessment)
		if err != nil {
			result.Issues = append(result.Issues, PublicationValidationIssue{Code: PublicationIssueAssessmentIncomplete, Path: use.path, Message: "This Assessment is incomplete and cannot be published."})
			continue
		}
		resolved[use.key] = struct{}{}
		result.Bindings = append(result.Bindings, binding)
	}
	sort.Slice(result.Bindings, func(i, j int) bool { return result.Bindings[i].AssessmentKey < result.Bindings[j].AssessmentKey })
	return result
}

func unavailableAssessmentIssue(path string) PublicationValidationIssue {
	return PublicationValidationIssue{Code: PublicationIssueAssessmentUnavailable, Path: path, Message: "This Assessment reference is not available for publication from this Review."}
}

func publicationAssessmentUses(snapshot ReviewSnapshot) []publicationAssessmentUse {
	uses := make([]publicationAssessmentUse, 0)
	for moduleIndex, module := range snapshot.Modules {
		for lessonIndex, lesson := range module.Lessons {
			for blockIndex, block := range lesson.Content.Blocks {
				if payload, ok := block.Payload.(courses.KnowledgeCheckBlockPayload); ok {
					uses = append(uses, publicationAssessmentUse{key: payload.AssessmentKey, path: fmt.Sprintf("modules[%d].lessons[%d].content.blocks[%d].payload.assessmentKey", moduleIndex, lessonIndex, blockIndex)})
				}
			}
		}
	}
	return uses
}

func publishedAssessmentBinding(source ReviewSnapshotAssessment) (courses.PublishedAssessmentBinding, error) {
	binding := courses.PublishedAssessmentBinding{AssessmentKey: source.AssessmentKey, Questions: make([]courses.PublishedAssessmentQuestion, 0, len(source.Questions))}
	for _, question := range source.Questions {
		published := courses.PublishedAssessmentQuestion{StableKey: question.StableKey, Type: courses.PublishedAssessmentQuestionType(question.Type), Prompt: question.Prompt, Position: question.Position, CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...), Options: []courses.PublishedAssessmentOption{}, LeftItems: []courses.PublishedAssessmentItem{}, RightItems: []courses.PublishedAssessmentItem{}, CorrectPairs: []courses.PublishedAssessmentPair{}}
		for _, option := range question.Options {
			published.Options = append(published.Options, courses.PublishedAssessmentOption{StableKey: option.StableKey, Text: option.Text, Position: option.Position})
		}
		for _, item := range question.LeftItems {
			published.LeftItems = append(published.LeftItems, courses.PublishedAssessmentItem{StableKey: item.StableKey, Text: item.Text, Position: item.Position})
		}
		for _, item := range question.RightItems {
			published.RightItems = append(published.RightItems, courses.PublishedAssessmentItem{StableKey: item.StableKey, Text: item.Text, Position: item.Position})
		}
		for _, pair := range question.CorrectPairs {
			published.CorrectPairs = append(published.CorrectPairs, courses.PublishedAssessmentPair{LeftKey: pair.LeftKey, RightKey: pair.RightKey})
		}
		binding.Questions = append(binding.Questions, published)
	}
	if binding.Validate() != nil {
		return courses.PublishedAssessmentBinding{}, assessments.ErrInvalidAssessment
	}
	return binding, nil
}
