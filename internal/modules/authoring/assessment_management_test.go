package authoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
)

type assessmentRepositoryFake struct {
	created    assessments.AssessmentInput
	assessment assessments.Assessment
	updateID   assessments.AssessmentID
	expected   int64
	updated    assessments.AssessmentUpdate
	summaries  []assessments.AssessmentSummary
	total      int
	createErr  error
	getErr     error
	updateErr  error
	listErr    error
}

func (f *assessmentRepositoryFake) CreateAssessment(_ context.Context, input assessments.AssessmentInput) (assessments.Assessment, error) {
	f.created = input
	if f.createErr != nil {
		return assessments.Assessment{}, f.createErr
	}
	return f.assessment, nil
}
func (f *assessmentRepositoryFake) GetAssessment(context.Context, assessments.AssessmentID) (assessments.Assessment, error) {
	if f.getErr != nil {
		return assessments.Assessment{}, f.getErr
	}
	return f.assessment, nil
}
func (f *assessmentRepositoryFake) UpdateAssessment(_ context.Context, id assessments.AssessmentID, expected int64, update assessments.AssessmentUpdate) (assessments.Assessment, error) {
	f.updateID, f.expected, f.updated = id, expected, update
	if f.updateErr != nil {
		return assessments.Assessment{}, f.updateErr
	}
	return f.assessment, nil
}
func (f *assessmentRepositoryFake) ListAssessmentSummariesForDraft(context.Context, string, int, int) ([]assessments.AssessmentSummary, int, error) {
	return f.summaries, f.total, f.listErr
}

func managedAssessment() assessments.Assessment {
	return assessments.Assessment{
		ID: "33333333-3333-4333-8333-333333333333", OwnerDraftID: string(testDraftA), CreatedByUserID: string(testActorID),
		Title: "Assessment", Revision: 1, CreatedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
	}
}

func TestAssessmentManagementUsesExactDraftCapabilityAndTrustedProvenance(t *testing.T) {
	actor := resolvedActor(t)
	for _, role := range []MemberRole{MemberAuthor, MemberMaintainer} {
		t.Run(string(role), func(t *testing.T) {
			repository := &assessmentRepositoryFake{assessment: managedAssessment()}
			service := NewAssessmentManagementService(repository, NewAuthorizationService(&membershipReaderFake{roles: map[DraftID]MemberRole{testDraftA: role}}))
			created, err := service.Create(context.Background(), actor, testDraftA, AssessmentDefinition{Title: "Assessment"})
			if err != nil || created.ID == "" || repository.created.OwnerDraftID != string(testDraftA) || repository.created.CreatedByUserID != string(actor.UserID()) {
				t.Fatalf("create trusted scope = %#v result=%#v err=%v", repository.created, created, err)
			}
			if _, err := service.Get(context.Background(), actor, testDraftA, created.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Update(context.Background(), actor, testDraftA, created.ID, 1, AssessmentDefinition{Title: "Assessment"}); err != nil || repository.expected != 1 {
				t.Fatalf("update=%v expected=%d", err, repository.expected)
			}
		})
	}
}

func TestAssessmentManagementHidesUnauthorizedAndForeignAssessments(t *testing.T) {
	actor := resolvedActor(t)
	for _, test := range []struct {
		name       string
		roles      map[DraftID]MemberRole
		assessment assessments.Assessment
	}{
		{name: "another draft membership", roles: map[DraftID]MemberRole{testDraftB: MemberAuthor}, assessment: managedAssessment()},
		{name: "global administrator has no Draft membership", roles: map[DraftID]MemberRole{}, assessment: managedAssessment()},
		{name: "foreign assessment", roles: map[DraftID]MemberRole{testDraftA: MemberAuthor}, assessment: func() assessments.Assessment {
			value := managedAssessment()
			value.OwnerDraftID = string(testDraftB)
			return value
		}()},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &assessmentRepositoryFake{assessment: test.assessment}
			service := NewAssessmentManagementService(repository, NewAuthorizationService(&membershipReaderFake{roles: test.roles}))
			if _, err := service.Get(context.Background(), actor, testDraftA, test.assessment.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("hidden result = %v", err)
			}
		})
	}
}
