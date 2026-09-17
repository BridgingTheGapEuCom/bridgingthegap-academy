package authoring

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type draftCreationRepositoryFake struct {
	draft       CourseDraft
	err         error
	calls       int
	input       DraftCreationInput
	createdByID string
}

func (r *draftCreationRepositoryFake) CreateDraftForCreator(_ context.Context, input DraftCreationInput, createdBy string) (CourseDraft, error) {
	r.calls++
	r.input = input
	r.createdByID = createdBy
	return r.draft, r.err
}

func validDraftCreationInput(t *testing.T) DraftCreationInput {
	t.Helper()
	version, err := courses.ParseVersion("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	return DraftCreationInput{
		IntendedVersion: version, SourceLanguage: "en", Title: "New course",
		Description: "A new editable Draft.", LearningObjectives: []string{"Explain the course"}, Changelog: "Initial Draft.",
	}
}

func TestDraftCreationServiceUsesOnlyAuthenticatedActorAndValidatedInput(t *testing.T) {
	repository := &draftCreationRepositoryFake{draft: mutationDraft()}
	service := NewDraftCreationService(repository)
	input := validDraftCreationInput(t)
	created, err := service.Create(context.Background(), resolvedActor(t), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != repository.draft.ID || repository.calls != 1 || repository.createdByID != string(testActorID) || !reflect.DeepEqual(repository.input, input) {
		t.Fatalf("creation did not use the authenticated actor and supplied input: %#v", repository)
	}

	input.LearningObjectives[0] = "caller mutation"
	if repository.input.LearningObjectives[0] != "Explain the course" {
		t.Fatal("creation retained a mutable caller objective slice")
	}
}

func TestDraftCreationServiceRejectsInvalidInputBeforeStorage(t *testing.T) {
	repository := &draftCreationRepositoryFake{}
	service := NewDraftCreationService(repository)
	input := validDraftCreationInput(t)
	input.Title = " "
	if _, err := service.Create(context.Background(), resolvedActor(t), input); !errors.Is(err, ErrInvalidDraftCreation) || repository.calls != 0 {
		t.Fatalf("invalid creation reached storage: %v calls=%d", err, repository.calls)
	}
	if _, err := NewDraftCreationService(nil).Create(context.Background(), resolvedActor(t), validDraftCreationInput(t)); !errors.Is(err, ErrInvalidDraftCreation) {
		t.Fatalf("unavailable creator = %v", err)
	}
}
