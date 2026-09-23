package assessments

import (
	"errors"
	"sort"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

// ValidateResponsesForPublishedAssessment validates learner response identity
// and shape against one immutable Courses-owned binding. Partial response sets
// are permitted for IN_PROGRESS attempts; submissions require every question.
func ValidateResponsesForPublishedAssessment(binding courses.PublishedAssessmentBinding, source []AttemptResponse, complete bool) ([]AttemptResponse, error) {
	if binding.Validate() != nil {
		return nil, ErrInvalidAttempt
	}
	responses, err := CanonicalAttemptResponses(source)
	if err != nil {
		return nil, err
	}
	questions := make(map[string]courses.PublishedAssessmentQuestion, len(binding.Questions))
	for _, question := range binding.Questions {
		questions[question.StableKey] = question
	}
	for _, response := range responses {
		question, exists := questions[response.QuestionKey]
		if !exists || !responseMatchesQuestion(response, question) {
			return nil, ErrInvalidAttempt
		}
	}
	if complete && len(responses) != len(binding.Questions) {
		return nil, ErrInvalidAttempt
	}
	if complete {
		for _, question := range binding.Questions {
			if _, exists := responseByQuestion(responses, question.StableKey); !exists {
				return nil, ErrInvalidAttempt
			}
		}
	}
	return responses, nil
}

// GradePublishedAssessment is pure: it grades a complete, already-validated
// learner response set against private immutable answer definitions.
func GradePublishedAssessment(binding courses.PublishedAssessmentBinding, source []AttemptResponse) (AttemptResult, error) {
	responses, err := ValidateResponsesForPublishedAssessment(binding, source, true)
	if err != nil {
		return AttemptResult{}, err
	}
	correct := 0
	for _, question := range binding.Questions {
		response, _ := responseByQuestion(responses, question.StableKey)
		if responseCorrect(response, question) {
			correct++
		}
	}
	result := AttemptResult{CorrectCount: correct, TotalCount: len(binding.Questions)}
	if err := result.Validate(); err != nil {
		return AttemptResult{}, errors.New("invalid immutable assessment result")
	}
	return result, nil
}

func responseMatchesQuestion(response AttemptResponse, question courses.PublishedAssessmentQuestion) bool {
	switch question.Type {
	case courses.PublishedQuestionSingleChoice:
		return response.Type == QuestionSingleChoice && optionExists(question.Options, response.SelectedOptionKey)
	case courses.PublishedQuestionMultipleChoice:
		if response.Type != QuestionMultipleChoice {
			return false
		}
		for _, key := range response.SelectedOptionKeys {
			if !optionExists(question.Options, key) {
				return false
			}
		}
		return true
	case courses.PublishedQuestionMatching:
		if response.Type != QuestionMatching {
			return false
		}
		left, right := assessmentItemKeys(question.LeftItems), assessmentItemKeys(question.RightItems)
		if len(response.Pairs) > len(left) {
			return false
		}
		for _, pair := range response.Pairs {
			if _, exists := left[pair.LeftItemKey]; !exists {
				return false
			}
			if _, exists := right[pair.RightItemKey]; !exists {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func responseCorrect(response AttemptResponse, question courses.PublishedAssessmentQuestion) bool {
	switch question.Type {
	case courses.PublishedQuestionSingleChoice:
		return response.SelectedOptionKey == question.CorrectOptionKeys[0]
	case courses.PublishedQuestionMultipleChoice:
		return equalStringSets(response.SelectedOptionKeys, question.CorrectOptionKeys)
	case courses.PublishedQuestionMatching:
		if len(response.Pairs) != len(question.CorrectPairs) {
			return false
		}
		pairs := make(map[string]string, len(response.Pairs))
		for _, pair := range response.Pairs {
			pairs[pair.LeftItemKey] = pair.RightItemKey
		}
		for _, pair := range question.CorrectPairs {
			if pairs[pair.LeftKey] != pair.RightKey {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func responseByQuestion(responses []AttemptResponse, key string) (AttemptResponse, bool) {
	index := sort.Search(len(responses), func(index int) bool { return responses[index].QuestionKey >= key })
	returnResponse := AttemptResponse{}
	if index < len(responses) && responses[index].QuestionKey == key {
		returnResponse = responses[index]
		return returnResponse, true
	}
	return returnResponse, false
}

func optionExists(options []courses.PublishedAssessmentOption, key string) bool {
	for _, option := range options {
		if option.StableKey == key {
			return true
		}
	}
	return false
}

func assessmentItemKeys(items []courses.PublishedAssessmentItem) map[string]struct{} {
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		keys[item.StableKey] = struct{}{}
	}
	return keys
}

func equalStringSets(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}
