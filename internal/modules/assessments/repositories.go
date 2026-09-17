package assessments

import "context"

// Repository owns complete mutable Assessment aggregates. Its writes are
// aggregate-atomic; callers cannot observe a partially replaced question set.
type Repository interface {
	CreateAssessment(context.Context, AssessmentInput) (Assessment, error)
	GetAssessment(context.Context, AssessmentID) (Assessment, error)
	UpdateAssessment(context.Context, AssessmentID, int64, AssessmentUpdate) (Assessment, error)
}
