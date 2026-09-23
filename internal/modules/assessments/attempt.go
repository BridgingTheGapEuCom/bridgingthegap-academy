package assessments

import (
	"sort"
	"time"
)

type AttemptID string
type AttemptState string

const (
	AttemptInProgress AttemptState = "IN_PROGRESS"
	AttemptSubmitted  AttemptState = "SUBMITTED"
)

// AssessmentAttempt is private learner data. It references the exact
// Courses-owned immutable binding through CourseVersionID and AssessmentKey;
// it never carries an Authoring Assessment or any authoritative answer key.
type AssessmentAttempt struct {
	ID              AttemptID
	LearnerUserID   string `json:"-"`
	CourseVersionID string `json:"-"`
	AssessmentKey   AssessmentID
	State           AttemptState
	Revision        int64
	Responses       []AttemptResponse `json:"-"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	SubmittedAt     *time.Time
	Result          *AttemptResult
}

// AttemptResult is the private aggregate score frozen with a submitted
// Attempt. It deliberately contains no answer keys or per-question feedback.
type AttemptResult struct {
	CorrectCount int
	TotalCount   int
}

func (result AttemptResult) Validate() error {
	if result.TotalCount < 1 || result.CorrectCount < 0 || result.CorrectCount > result.TotalCount {
		return ErrInvalidAttempt
	}
	return nil
}

type AttemptInput struct {
	LearnerUserID   string `json:"-"`
	CourseVersionID string `json:"-"`
	AssessmentKey   AssessmentID
	Responses       []AttemptResponse `json:"-"`
}

type AttemptUpdate struct {
	Responses []AttemptResponse `json:"-"`
}

// AttemptResponse is a tagged learner response. Stable keys are the only
// semantic identifiers; array positions are never persisted as answers.
type AttemptResponse struct {
	QuestionKey        string
	Type               QuestionType
	SelectedOptionKey  string
	SelectedOptionKeys []string
	Pairs              []AttemptMatchingPair
}

type AttemptMatchingPair struct {
	LeftItemKey  string
	RightItemKey string
}

func ParseAttemptID(value string) (AttemptID, error) {
	if !validUUID(value) {
		return "", ErrInvalidAttempt
	}
	return AttemptID(value), nil
}

func (input AttemptInput) Validate() error {
	if !validUUID(input.LearnerUserID) || !validUUID(input.CourseVersionID) || !validUUID(string(input.AssessmentKey)) || validateAttemptResponses(input.Responses) != nil {
		return ErrInvalidAttempt
	}
	return nil
}

func (update AttemptUpdate) Validate() error {
	if validateAttemptResponses(update.Responses) != nil {
		return ErrInvalidAttempt
	}
	return nil
}

func (attempt AssessmentAttempt) Validate() error {
	if !validUUID(string(attempt.ID)) || !validUUID(attempt.LearnerUserID) || !validUUID(attempt.CourseVersionID) || !validUUID(string(attempt.AssessmentKey)) || attempt.Revision < 1 || attempt.CreatedAt.IsZero() || attempt.UpdatedAt.IsZero() || validateAttemptResponses(attempt.Responses) != nil {
		return ErrInvalidAttempt
	}
	switch attempt.State {
	case AttemptInProgress:
		if attempt.SubmittedAt != nil || attempt.Result != nil {
			return ErrInvalidAttempt
		}
	case AttemptSubmitted:
		if attempt.SubmittedAt == nil || attempt.SubmittedAt.IsZero() || attempt.SubmittedAt.Before(attempt.CreatedAt) || attempt.Result == nil || attempt.Result.Validate() != nil {
			return ErrInvalidAttempt
		}
	default:
		return ErrInvalidAttempt
	}
	return nil
}

// CanonicalAttemptResponses gives the repository and future API layer one
// deterministic representation. It does not check question membership; only
// a binding-aware submission service can do that safely.
func CanonicalAttemptResponses(source []AttemptResponse) ([]AttemptResponse, error) {
	result := make([]AttemptResponse, 0, len(source))
	for _, response := range source {
		copy := AttemptResponse{
			QuestionKey:        response.QuestionKey,
			Type:               response.Type,
			SelectedOptionKey:  response.SelectedOptionKey,
			SelectedOptionKeys: append([]string(nil), response.SelectedOptionKeys...),
			Pairs:              append([]AttemptMatchingPair(nil), response.Pairs...),
		}
		sort.Strings(copy.SelectedOptionKeys)
		sort.Slice(copy.Pairs, func(i, j int) bool {
			if copy.Pairs[i].LeftItemKey == copy.Pairs[j].LeftItemKey {
				return copy.Pairs[i].RightItemKey < copy.Pairs[j].RightItemKey
			}
			return copy.Pairs[i].LeftItemKey < copy.Pairs[j].LeftItemKey
		})
		result = append(result, copy)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].QuestionKey < result[j].QuestionKey })
	if validateAttemptResponses(result) != nil {
		return nil, ErrInvalidAttempt
	}
	return result, nil
}

func (attempt AssessmentAttempt) WithResponses(responses []AttemptResponse, updatedAt time.Time) (AssessmentAttempt, error) {
	if attempt.State != AttemptInProgress || updatedAt.IsZero() {
		return AssessmentAttempt{}, ErrAttemptImmutable
	}
	canonical, err := CanonicalAttemptResponses(responses)
	if err != nil {
		return AssessmentAttempt{}, err
	}
	updated := attempt
	updated.Responses = canonical
	updated.Revision++
	updated.UpdatedAt = updatedAt.UTC()
	if err := updated.Validate(); err != nil {
		return AssessmentAttempt{}, err
	}
	return updated, nil
}

func (attempt AssessmentAttempt) Submit(result AttemptResult, submittedAt time.Time) (AssessmentAttempt, error) {
	if attempt.State != AttemptInProgress || submittedAt.IsZero() || result.Validate() != nil {
		return AssessmentAttempt{}, ErrAttemptImmutable
	}
	submitted := attempt
	at := submittedAt.UTC()
	submitted.State = AttemptSubmitted
	submitted.SubmittedAt = &at
	submitted.Result = &AttemptResult{CorrectCount: result.CorrectCount, TotalCount: result.TotalCount}
	submitted.UpdatedAt = at
	submitted.Revision++
	if err := submitted.Validate(); err != nil {
		return AssessmentAttempt{}, err
	}
	return submitted, nil
}

func validateAttemptResponses(responses []AttemptResponse) error {
	if len(responses) > 100 {
		return ErrInvalidAttempt
	}
	previousQuestionKey := ""
	for _, response := range responses {
		if !validStableKey(response.QuestionKey) || previousQuestionKey != "" && response.QuestionKey <= previousQuestionKey {
			return ErrInvalidAttempt
		}
		previousQuestionKey = response.QuestionKey
		switch response.Type {
		case QuestionSingleChoice:
			if !validStableKey(response.SelectedOptionKey) || len(response.SelectedOptionKeys) != 0 || len(response.Pairs) != 0 {
				return ErrInvalidAttempt
			}
		case QuestionMultipleChoice:
			if response.SelectedOptionKey != "" || len(response.SelectedOptionKeys) == 0 || len(response.Pairs) != 0 || !strictStableKeys(response.SelectedOptionKeys) {
				return ErrInvalidAttempt
			}
		case QuestionMatching:
			if response.SelectedOptionKey != "" || len(response.SelectedOptionKeys) != 0 || len(response.Pairs) == 0 || !strictMatchingPairs(response.Pairs) {
				return ErrInvalidAttempt
			}
		default:
			return ErrInvalidAttempt
		}
	}
	return nil
}

func strictStableKeys(keys []string) bool {
	previous := ""
	for _, key := range keys {
		if !validStableKey(key) || previous != "" && key <= previous {
			return false
		}
		previous = key
	}
	return true
}

func strictMatchingPairs(pairs []AttemptMatchingPair) bool {
	leftKeys, rightKeys := map[string]struct{}{}, map[string]struct{}{}
	previous := ""
	for _, pair := range pairs {
		if !validStableKey(pair.LeftItemKey) || !validStableKey(pair.RightItemKey) || previous != "" && pair.LeftItemKey <= previous {
			return false
		}
		if _, duplicate := leftKeys[pair.LeftItemKey]; duplicate {
			return false
		}
		if _, duplicate := rightKeys[pair.RightItemKey]; duplicate {
			return false
		}
		leftKeys[pair.LeftItemKey], rightKeys[pair.RightItemKey] = struct{}{}, struct{}{}
		previous = pair.LeftItemKey
	}
	return true
}
