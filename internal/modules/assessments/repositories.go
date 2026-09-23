package assessments

import (
	"context"
	"time"
)

type AssessmentSummary struct {
	ID            AssessmentID
	Title         string
	QuestionCount int
	Revision      int64
	UpdatedAt     time.Time
}

// Repository owns complete mutable Assessment aggregates. Its writes are
// aggregate-atomic; callers cannot observe a partially replaced question set.
type Repository interface {
	CreateAssessment(context.Context, AssessmentInput) (Assessment, error)
	GetAssessment(context.Context, AssessmentID) (Assessment, error)
	UpdateAssessment(context.Context, AssessmentID, int64, AssessmentUpdate) (Assessment, error)
	ListAssessmentSummariesForDraft(context.Context, string, int, int) ([]AssessmentSummary, int, error)
}

// AttemptRepository owns private learner response aggregates. It deliberately
// does not expose learner history or grading queries in this foundation slice.
type AttemptRepository interface {
	CreateAssessmentAttempt(context.Context, AttemptInput) (AssessmentAttempt, error)
	GetAssessmentAttempt(context.Context, AttemptID) (AssessmentAttempt, error)
	UpdateAssessmentAttempt(context.Context, AttemptID, int64, AttemptUpdate) (AssessmentAttempt, error)
	SubmitAssessmentAttempt(context.Context, AttemptID, int64, AttemptResult, time.Time) (AssessmentAttempt, error)
}
