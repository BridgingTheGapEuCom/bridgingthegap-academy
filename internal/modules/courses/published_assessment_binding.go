package courses

import "strings"

type PublishedAssessmentQuestionType string

const (
	PublishedQuestionSingleChoice   PublishedAssessmentQuestionType = "SINGLE_CHOICE"
	PublishedQuestionMultipleChoice PublishedAssessmentQuestionType = "MULTIPLE_CHOICE"
	PublishedQuestionMatching       PublishedAssessmentQuestionType = "MATCHING"
)

// PublishedAssessmentBinding freezes the learner-visible definition and the
// private answer key for one canonical assessmentKey. Public read models must
// map this type explicitly and must never serialize it directly.
type PublishedAssessmentBinding struct {
	AssessmentKey string
	Questions     []PublishedAssessmentQuestion `json:"-"`
}

type PublishedAssessmentQuestion struct {
	StableKey         string
	Type              PublishedAssessmentQuestionType
	Prompt            string
	Position          int
	Options           []PublishedAssessmentOption `json:"-"`
	CorrectOptionKeys []string                    `json:"-"`
	LeftItems         []PublishedAssessmentItem   `json:"-"`
	RightItems        []PublishedAssessmentItem   `json:"-"`
	CorrectPairs      []PublishedAssessmentPair   `json:"-"`
}

type PublishedAssessmentOption struct {
	StableKey string
	Text      string
	Position  int
}

type PublishedAssessmentItem struct {
	StableKey string
	Text      string
	Position  int
}

type PublishedAssessmentPair struct {
	LeftKey  string
	RightKey string
}

func (b PublishedAssessmentBinding) Validate() error {
	if !uuidPattern.MatchString(b.AssessmentKey) || len(b.Questions) == 0 || len(b.Questions) > 100 {
		return ErrInvalidImmutableCourseVersion
	}
	keys := make(map[string]struct{}, len(b.Questions))
	for position, question := range b.Questions {
		if question.Position != position || !validPublishedAssessmentKey(question.StableKey) || !validPublishedAssessmentText(question.Prompt, 10000) {
			return ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := keys[question.StableKey]; duplicate {
			return ErrInvalidImmutableCourseVersion
		}
		keys[question.StableKey] = struct{}{}
		if question.validate() != nil {
			return ErrInvalidImmutableCourseVersion
		}
	}
	return nil
}

func (q PublishedAssessmentQuestion) validate() error {
	switch q.Type {
	case PublishedQuestionSingleChoice:
		if len(q.CorrectOptionKeys) != 1 || len(q.LeftItems) != 0 || len(q.RightItems) != 0 || len(q.CorrectPairs) != 0 {
			return ErrInvalidImmutableCourseVersion
		}
		return validatePublishedOptions(q.Options, q.CorrectOptionKeys)
	case PublishedQuestionMultipleChoice:
		if len(q.CorrectOptionKeys) == 0 || len(q.LeftItems) != 0 || len(q.RightItems) != 0 || len(q.CorrectPairs) != 0 {
			return ErrInvalidImmutableCourseVersion
		}
		return validatePublishedOptions(q.Options, q.CorrectOptionKeys)
	case PublishedQuestionMatching:
		if len(q.Options) != 0 || len(q.CorrectOptionKeys) != 0 {
			return ErrInvalidImmutableCourseVersion
		}
		return validatePublishedMatching(q.LeftItems, q.RightItems, q.CorrectPairs)
	default:
		return ErrInvalidImmutableCourseVersion
	}
}

func validatePublishedOptions(options []PublishedAssessmentOption, correct []string) error {
	if len(options) < 2 || len(options) > 20 {
		return ErrInvalidImmutableCourseVersion
	}
	keys := make(map[string]struct{}, len(options))
	for position, option := range options {
		if option.Position != position || !validPublishedAssessmentKey(option.StableKey) || !validPublishedAssessmentText(option.Text, 4000) {
			return ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := keys[option.StableKey]; duplicate {
			return ErrInvalidImmutableCourseVersion
		}
		keys[option.StableKey] = struct{}{}
	}
	seen := make(map[string]struct{}, len(correct))
	for _, key := range correct {
		if _, exists := keys[key]; !exists {
			return ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidImmutableCourseVersion
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validatePublishedMatching(left, right []PublishedAssessmentItem, pairs []PublishedAssessmentPair) error {
	if len(left) == 0 || len(left) > 20 || len(left) != len(right) || len(pairs) != len(left) {
		return ErrInvalidImmutableCourseVersion
	}
	leftKeys, err := publishedAssessmentItemKeys(left)
	if err != nil {
		return err
	}
	rightKeys, err := publishedAssessmentItemKeys(right)
	if err != nil {
		return err
	}
	usedLeft, usedRight := map[string]struct{}{}, map[string]struct{}{}
	for _, pair := range pairs {
		if _, exists := leftKeys[pair.LeftKey]; !exists {
			return ErrInvalidImmutableCourseVersion
		}
		if _, exists := rightKeys[pair.RightKey]; !exists {
			return ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := usedLeft[pair.LeftKey]; duplicate {
			return ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := usedRight[pair.RightKey]; duplicate {
			return ErrInvalidImmutableCourseVersion
		}
		usedLeft[pair.LeftKey], usedRight[pair.RightKey] = struct{}{}, struct{}{}
	}
	return nil
}

func publishedAssessmentItemKeys(items []PublishedAssessmentItem) (map[string]struct{}, error) {
	keys := make(map[string]struct{}, len(items))
	for position, item := range items {
		if item.Position != position || !validPublishedAssessmentKey(item.StableKey) || !validPublishedAssessmentText(item.Text, 4000) {
			return nil, ErrInvalidImmutableCourseVersion
		}
		if _, duplicate := keys[item.StableKey]; duplicate {
			return nil, ErrInvalidImmutableCourseVersion
		}
		keys[item.StableKey] = struct{}{}
	}
	return keys, nil
}

func validPublishedAssessmentKey(value string) bool {
	if len(value) < 1 || len(value) > 160 {
		return false
	}
	for index, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' && index > 0 && index < len(value)-1 {
			continue
		}
		return false
	}
	return !strings.Contains(value, "--")
}

func validPublishedAssessmentText(value string, maximum int) bool {
	return value == strings.TrimSpace(value) && len(value) > 0 && len(value) <= maximum
}

func contentAssessmentReferences(content LessonContent) []string {
	result := make([]string, 0)
	for _, block := range content.Blocks {
		if payload, ok := block.Payload.(KnowledgeCheckBlockPayload); ok {
			result = append(result, payload.AssessmentKey)
		}
	}
	return result
}
