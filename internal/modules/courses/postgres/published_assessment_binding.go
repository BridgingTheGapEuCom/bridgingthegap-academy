package postgres

import (
	"encoding/json"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type persistedAssessmentDefinition struct {
	Questions []persistedAssessmentQuestion `json:"questions"`
}

type persistedAssessmentQuestion struct {
	StableKey         string                                  `json:"stableKey"`
	Type              courses.PublishedAssessmentQuestionType `json:"type"`
	Prompt            string                                  `json:"prompt"`
	Position          int                                     `json:"position"`
	Options           []courses.PublishedAssessmentOption     `json:"options,omitempty"`
	CorrectOptionKeys []string                                `json:"correctOptionKeys,omitempty"`
	LeftItems         []courses.PublishedAssessmentItem       `json:"leftItems,omitempty"`
	RightItems        []courses.PublishedAssessmentItem       `json:"rightItems,omitempty"`
	CorrectPairs      []courses.PublishedAssessmentPair       `json:"correctPairs,omitempty"`
}

func marshalPublishedAssessment(binding courses.PublishedAssessmentBinding) ([]byte, error) {
	if binding.Validate() != nil {
		return nil, courses.ErrInvalidImmutableCourseVersion
	}
	definition := persistedAssessmentDefinition{Questions: make([]persistedAssessmentQuestion, 0, len(binding.Questions))}
	for _, question := range binding.Questions {
		definition.Questions = append(definition.Questions, persistedAssessmentQuestion{
			StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
			Options: append([]courses.PublishedAssessmentOption(nil), question.Options...), CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...),
			LeftItems: append([]courses.PublishedAssessmentItem(nil), question.LeftItems...), RightItems: append([]courses.PublishedAssessmentItem(nil), question.RightItems...), CorrectPairs: append([]courses.PublishedAssessmentPair(nil), question.CorrectPairs...),
		})
	}
	return json.Marshal(definition)
}

func unmarshalPublishedAssessment(key string, encoded []byte) (courses.PublishedAssessmentBinding, error) {
	var definition persistedAssessmentDefinition
	if err := json.Unmarshal(encoded, &definition); err != nil || definition.Questions == nil {
		return courses.PublishedAssessmentBinding{}, courses.ErrInvalidImmutableCourseVersion
	}
	binding := courses.PublishedAssessmentBinding{AssessmentKey: key, Questions: make([]courses.PublishedAssessmentQuestion, 0, len(definition.Questions))}
	for _, question := range definition.Questions {
		binding.Questions = append(binding.Questions, courses.PublishedAssessmentQuestion{
			StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
			Options: append([]courses.PublishedAssessmentOption(nil), question.Options...), CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...),
			LeftItems: append([]courses.PublishedAssessmentItem(nil), question.LeftItems...), RightItems: append([]courses.PublishedAssessmentItem(nil), question.RightItems...), CorrectPairs: append([]courses.PublishedAssessmentPair(nil), question.CorrectPairs...),
		})
	}
	if binding.Validate() != nil {
		return courses.PublishedAssessmentBinding{}, courses.ErrInvalidImmutableCourseVersion
	}
	return binding, nil
}
