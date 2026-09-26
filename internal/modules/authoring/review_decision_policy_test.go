package authoring

import (
	"errors"
	"testing"
)

func TestReviewDecisionPolicyIndependentReviewerRule(t *testing.T) {
	for _, test := range []struct {
		name     string
		required bool
		input    ReviewDecisionPolicyInput
		want     error
	}{
		{
			name:     "enabled rejects submitter",
			required: true,
			input:    ReviewDecisionPolicyInput{DecisionActorUserID: "user-a", ReviewSubmittedByUserID: "user-a"},
			want:     ErrIndependentReviewerRequired,
		},
		{
			name:     "enabled allows another actor",
			required: true,
			input:    ReviewDecisionPolicyInput{DecisionActorUserID: "user-b", ReviewSubmittedByUserID: "user-a"},
		},
		{
			name:     "disabled allows submitter",
			required: false,
			input:    ReviewDecisionPolicyInput{DecisionActorUserID: "user-a", ReviewSubmittedByUserID: "user-a"},
		},
		{
			name:     "disabled allows another actor",
			required: false,
			input:    ReviewDecisionPolicyInput{DecisionActorUserID: "user-b", ReviewSubmittedByUserID: "user-a"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := NewReviewDecisionPolicy(test.required).Check(test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("Check() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestReviewDecisionPolicyUsesEachCycleSubmitterProvenance(t *testing.T) {
	policy := NewReviewDecisionPolicy(true)
	if err := policy.Check(ReviewDecisionPolicyInput{DecisionActorUserID: "reviewer", ReviewSubmittedByUserID: "submitter-a"}); err != nil {
		t.Fatalf("different cycle submitter unexpectedly denied: %v", err)
	}
	if err := policy.Check(ReviewDecisionPolicyInput{DecisionActorUserID: "reviewer", ReviewSubmittedByUserID: "reviewer"}); !errors.Is(err, ErrIndependentReviewerRequired) {
		t.Fatalf("same cycle submitter allowed: %v", err)
	}
}

func TestReviewDecisionPolicyRejectsMissingAuthoritativeProvenance(t *testing.T) {
	policy := NewReviewDecisionPolicy(false)
	for _, input := range []ReviewDecisionPolicyInput{
		{ReviewSubmittedByUserID: "submitter"},
		{DecisionActorUserID: "reviewer"},
		{DecisionActorUserID: " ", ReviewSubmittedByUserID: "submitter"},
	} {
		if err := policy.Check(input); !errors.Is(err, ErrReviewDecisionPolicyInput) {
			t.Fatalf("missing authoritative input allowed: %v", err)
		}
	}
}
