package authoring

import (
	"errors"
	"strings"
)

var (
	// ErrIndependentReviewerRequired means the configured policy forbids a
	// Review submitter from deciding their own submitted cycle.
	ErrIndependentReviewerRequired = errors.New("independent reviewer required")
	ErrReviewDecisionPolicyInput   = errors.New("invalid review decision policy input")
)

// ReviewDecisionPolicy is deliberately separate from resource-scoped
// authorization. It evaluates only trusted decision and immutable submission
// provenance supplied by the application layer.
type ReviewDecisionPolicy interface {
	Check(ReviewDecisionPolicyInput) error
}

// ReviewDecisionPolicyInput carries only the opaque server-authoritative IDs
// needed by an independence rule. It contains no roles, browser claims, or
// persistence dependencies.
type ReviewDecisionPolicyInput struct {
	DecisionActorUserID     string
	ReviewSubmittedByUserID string
}

type independentReviewPolicy struct {
	required bool
}

// NewReviewDecisionPolicy creates the deployment-configured independence
// policy applied after review.decide authorization and before persistence.
func NewReviewDecisionPolicy(requireIndependentReview bool) ReviewDecisionPolicy {
	return independentReviewPolicy{required: requireIndependentReview}
}

func (p independentReviewPolicy) Check(input ReviewDecisionPolicyInput) error {
	if strings.TrimSpace(input.DecisionActorUserID) == "" || strings.TrimSpace(input.ReviewSubmittedByUserID) == "" {
		return ErrReviewDecisionPolicyInput
	}
	if p.required && input.DecisionActorUserID == input.ReviewSubmittedByUserID {
		return ErrIndependentReviewerRequired
	}
	return nil
}
