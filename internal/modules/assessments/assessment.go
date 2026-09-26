package assessments

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type AssessmentID string
type QuestionType string

const (
	QuestionSingleChoice   QuestionType = "SINGLE_CHOICE"
	QuestionMultipleChoice QuestionType = "MULTIPLE_CHOICE"
	QuestionMatching       QuestionType = "MATCHING"
)

var (
	uuidPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	stableKeyPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

	ErrInvalidAssessment  = errors.New("invalid assessment")
	ErrAssessmentNotFound = errors.New("assessment record not found")
	ErrAssessmentConflict = errors.New("assessment record conflicts with existing data")
	ErrRevisionMismatch   = errors.New("assessment revision mismatch")
	ErrInvalidAttempt     = errors.New("invalid assessment attempt")
	ErrAttemptNotFound    = errors.New("assessment attempt record not found")
	ErrAttemptConflict    = errors.New("assessment attempt record conflicts with existing data")
	ErrAttemptImmutable   = errors.New("submitted assessment attempt is immutable")
	ErrAttemptUnavailable = errors.New("published assessment attempt context unavailable")
)

// Assessment is a mutable Authoring-owned definition. OwnerDraftID,
// CreatedByUserID, and Questions are deliberately excluded from default JSON
// serialization: future learner projections must explicitly choose what they
// expose and must never inherit answer keys by accident.
type Assessment struct {
	ID              AssessmentID
	OwnerDraftID    string `json:"-"`
	Title           string
	Revision        int64
	Questions       []Question `json:"-"`
	CreatedByUserID string     `json:"-"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AssessmentInput struct {
	OwnerDraftID    string `json:"-"`
	Title           string
	Questions       []Question `json:"-"`
	CreatedByUserID string     `json:"-"`
}

type AssessmentUpdate struct {
	Title     string
	Questions []Question `json:"-"`
}

// Question has an Assessment-local stable key. Position is display order and
// is intentionally independent from every stable key and answer reference.
type Question struct {
	StableKey         string
	Type              QuestionType
	Prompt            string
	Position          int
	Options           []ChoiceOption `json:"-"`
	CorrectOptionKeys []string       `json:"-"`
	LeftItems         []MatchingItem `json:"-"`
	RightItems        []MatchingItem `json:"-"`
	CorrectPairs      []MatchingPair `json:"-"`
}

type ChoiceOption struct {
	StableKey string
	Text      string
	Position  int
}

type MatchingItem struct {
	StableKey string
	Text      string
	Position  int
}

type MatchingPair struct {
	LeftKey  string
	RightKey string
}

func ParseAssessmentID(value string) (AssessmentID, error) {
	if !validUUID(value) {
		return "", ErrInvalidAssessment
	}
	return AssessmentID(value), nil
}

func (input AssessmentInput) Validate() error {
	if !validUUID(input.OwnerDraftID) || !validUUID(input.CreatedByUserID) || !validTitle(input.Title) || validateQuestions(input.Questions) != nil {
		return ErrInvalidAssessment
	}
	return nil
}

func (update AssessmentUpdate) Validate() error {
	if !validTitle(update.Title) || validateQuestions(update.Questions) != nil {
		return ErrInvalidAssessment
	}
	return nil
}

func (assessment Assessment) Validate() error {
	if !validUUID(string(assessment.ID)) || !validUUID(assessment.OwnerDraftID) || !validUUID(assessment.CreatedByUserID) || assessment.Revision < 1 || assessment.CreatedAt.IsZero() || assessment.UpdatedAt.IsZero() || !validTitle(assessment.Title) || validateQuestions(assessment.Questions) != nil {
		return ErrInvalidAssessment
	}
	return nil
}

func validateQuestions(questions []Question) error {
	// Empty definitions are valid mutable Authoring state. Publication later
	// decides whether a frozen Assessment is complete enough to publish.
	if len(questions) > 100 {
		return ErrInvalidAssessment
	}
	keys := make(map[string]struct{}, len(questions))
	for position, question := range questions {
		if question.Position != position || !validStableKey(question.StableKey) || !validText(question.Prompt, 10000) {
			return ErrInvalidAssessment
		}
		if _, exists := keys[question.StableKey]; exists {
			return ErrInvalidAssessment
		}
		keys[question.StableKey] = struct{}{}
		if err := question.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (question Question) Validate() error {
	switch question.Type {
	case QuestionSingleChoice:
		if len(question.LeftItems) != 0 || len(question.RightItems) != 0 || len(question.CorrectPairs) != 0 || len(question.CorrectOptionKeys) != 1 {
			return ErrInvalidAssessment
		}
		return validateChoices(question.Options, question.CorrectOptionKeys)
	case QuestionMultipleChoice:
		if len(question.LeftItems) != 0 || len(question.RightItems) != 0 || len(question.CorrectPairs) != 0 || len(question.CorrectOptionKeys) == 0 {
			return ErrInvalidAssessment
		}
		return validateChoices(question.Options, question.CorrectOptionKeys)
	case QuestionMatching:
		if len(question.Options) != 0 || len(question.CorrectOptionKeys) != 0 {
			return ErrInvalidAssessment
		}
		return validateMatching(question.LeftItems, question.RightItems, question.CorrectPairs)
	default:
		return ErrInvalidAssessment
	}
}

func validateChoices(options []ChoiceOption, correctKeys []string) error {
	if len(options) < 2 || len(options) > 20 {
		return ErrInvalidAssessment
	}
	keys := make(map[string]struct{}, len(options))
	for position, option := range options {
		if option.Position != position || !validStableKey(option.StableKey) || !validText(option.Text, 4000) {
			return ErrInvalidAssessment
		}
		if _, exists := keys[option.StableKey]; exists {
			return ErrInvalidAssessment
		}
		keys[option.StableKey] = struct{}{}
	}
	seenCorrect := make(map[string]struct{}, len(correctKeys))
	for _, key := range correctKeys {
		if _, exists := keys[key]; !exists {
			return ErrInvalidAssessment
		}
		if _, duplicate := seenCorrect[key]; duplicate {
			return ErrInvalidAssessment
		}
		seenCorrect[key] = struct{}{}
	}
	return nil
}

func validateMatching(left, right []MatchingItem, pairs []MatchingPair) error {
	if len(left) == 0 || len(left) > 20 || len(left) != len(right) || len(pairs) != len(left) {
		return ErrInvalidAssessment
	}
	leftKeys, err := matchingKeys(left)
	if err != nil {
		return err
	}
	rightKeys, err := matchingKeys(right)
	if err != nil {
		return err
	}
	usedLeft, usedRight := map[string]struct{}{}, map[string]struct{}{}
	for _, pair := range pairs {
		if _, exists := leftKeys[pair.LeftKey]; !exists {
			return ErrInvalidAssessment
		}
		if _, exists := rightKeys[pair.RightKey]; !exists {
			return ErrInvalidAssessment
		}
		if _, duplicate := usedLeft[pair.LeftKey]; duplicate {
			return ErrInvalidAssessment
		}
		if _, duplicate := usedRight[pair.RightKey]; duplicate {
			return ErrInvalidAssessment
		}
		usedLeft[pair.LeftKey], usedRight[pair.RightKey] = struct{}{}, struct{}{}
	}
	return nil
}

func matchingKeys(items []MatchingItem) (map[string]struct{}, error) {
	keys := make(map[string]struct{}, len(items))
	for position, item := range items {
		if item.Position != position || !validStableKey(item.StableKey) || !validText(item.Text, 4000) {
			return nil, ErrInvalidAssessment
		}
		if _, duplicate := keys[item.StableKey]; duplicate {
			return nil, ErrInvalidAssessment
		}
		keys[item.StableKey] = struct{}{}
	}
	return keys, nil
}

func validUUID(value string) bool { return uuidPattern.MatchString(value) }
func validStableKey(value string) bool {
	return len(value) >= 1 && len(value) <= 160 && stableKeyPattern.MatchString(value)
}
func validTitle(value string) bool { return validText(value, 240) }
func validText(value string, maximum int) bool {
	return value == strings.TrimSpace(value) && len(value) > 0 && len(value) <= maximum
}
