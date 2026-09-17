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
