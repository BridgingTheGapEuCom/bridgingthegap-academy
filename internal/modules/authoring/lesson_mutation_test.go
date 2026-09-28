package authoring

import (
	"context"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type lessonMutationRepositoryFake struct {
	draft        CourseDraft
	lessons      map[LessonID]DraftLesson
	createErrors []error
}

func (r *lessonMutationRepositoryFake) CreateGeneratedLesson(_ context.Context, draftID DraftID, moduleID ModuleID, expected int64, input LessonInput) (DraftLesson, CourseDraft, error) {
	if len(r.createErrors) > 0 {
		err := r.createErrors[0]
		r.createErrors = r.createErrors[1:]
		return DraftLesson{}, CourseDraft{}, err
	}
	if draftID != r.draft.ID || expected != r.draft.Revision {
		return DraftLesson{}, CourseDraft{}, ErrRevisionMismatch
	}
	for _, lesson := range r.lessons {
		if lesson.StableKey == input.StableKey {
			return DraftLesson{}, CourseDraft{}, ErrStableKeyCollision
		}
	}
	position := 0
	for _, lesson := range r.lessons {
		if lesson.ModuleID == moduleID {
			position++
		}
	}
	input.Position = position
	lesson := DraftLesson{ID: LessonID("id-" + input.StableKey), LessonInput: input, Revision: 1}
	r.lessons[lesson.ID] = lesson
	r.draft.Revision++
	return lesson, r.draft, nil
}

func (r *lessonMutationRepositoryFake) UpdateLessonMetadataForDraft(_ context.Context, draftID DraftID, lessonID LessonID, expected int64, patch DraftLessonPatch) (DraftLesson, CourseDraft, error) {
	lesson, found := r.lessons[lessonID]
	if !found || lesson.DraftID != draftID {
		return DraftLesson{}, CourseDraft{}, ErrNotFound
	}
	if lesson.Revision != expected {
		return DraftLesson{}, CourseDraft{}, ErrRevisionMismatch
	}
	next, err := patch.Apply(lesson.LessonInput)
	if err != nil {
		return DraftLesson{}, CourseDraft{}, err
	}
	lesson.LessonInput, lesson.Revision = next, lesson.Revision+1
	r.lessons[lesson.ID] = lesson
	r.draft.Revision++
	return lesson, r.draft, nil
}

func (r *lessonMutationRepositoryFake) ReorderLessonsForDraft(_ context.Context, draftID DraftID, expected int64, order []ModuleLessonOrder) (CourseDraft, error) {
	if draftID != r.draft.ID || expected != r.draft.Revision {
		return CourseDraft{}, ErrRevisionMismatch
	}
	for _, module := range order {
		for position, id := range module.LessonIDs {
			lesson, found := r.lessons[id]
			if !found {
				return CourseDraft{}, ErrInvalidStructure
			}
			lesson.ModuleID, lesson.Position = module.ModuleID, position
			r.lessons[id] = lesson
		}
	}
	r.draft.Revision++
	return r.draft, nil
}

func (r *lessonMutationRepositoryFake) ReplaceLessonPrerequisitesForDraft(context.Context, DraftID, LessonID, int64, []string) (DraftLesson, CourseDraft, error) {
	return DraftLesson{}, CourseDraft{}, ErrInvalidStructure
}

func (r *lessonMutationRepositoryFake) DeleteLessonForDraft(context.Context, DraftID, LessonID, int64, int64) (CourseDraft, error) {
	return CourseDraft{}, ErrInvalidStructure
}

func TestLessonMutationServiceGeneratesImmutableKeysAndPreservesThemOnMove(t *testing.T) {
	draft := mutationDraft()
	repository := &lessonMutationRepositoryFake{draft: draft, lessons: map[LessonID]DraftLesson{}, createErrors: []error{ErrStableKeyCollision}}
	service := NewLessonMutationService(repository, &draftMutationAuthorizerFake{})
	keys := []string{"lesson-collision", "lesson-first", "lesson-second"}
	service.generateStableKey = func() (string, error) {
		key := keys[0]
		keys = keys[1:]
		return key, nil
	}
	input := LessonCreateInput{DraftID: draft.ID, ModuleID: "module-a", Title: "First lesson", Description: "A lesson.", LearningObjectives: []string{"Understand the model"}}
	first, err := service.CreateLesson(context.Background(), resolvedActor(t), draft.ID, "module-a", draft.Revision, input)
	if err != nil || first.Lesson.StableKey != "lesson-first" || first.Lesson.Position != 0 || first.Draft.Revision != 4 {
		t.Fatalf("generated first Lesson = %#v err=%v", first, err)
	}
	secondInput := input
	secondInput.Title = "Second lesson"
	second, err := service.CreateLesson(context.Background(), resolvedActor(t), draft.ID, "module-a", first.Draft.Revision, secondInput)
	if err != nil || second.Lesson.StableKey != "lesson-second" || second.Lesson.Position != 1 || len(keys) != 0 {
		t.Fatalf("generated second Lesson = %#v err=%v keys=%#v", second, err, keys)
	}
	description := "Updated without replacing identity."
	updated, err := service.UpdateLesson(context.Background(), resolvedActor(t), draft.ID, first.Lesson.ID, first.Lesson.Revision, DraftLessonPatch{Description: &description})
	if err != nil || updated.Lesson.StableKey != first.Lesson.StableKey {
		t.Fatalf("Lesson update changed key: %#v err=%v", updated, err)
	}
	if _, err := service.ReorderLessons(context.Background(), resolvedActor(t), draft.ID, updated.Draft.Revision, []ModuleLessonOrder{{ModuleID: "module-a", LessonIDs: []LessonID{second.Lesson.ID}}, {ModuleID: "module-b", LessonIDs: []LessonID{first.Lesson.ID}}}); err != nil {
		t.Fatalf("move Lesson = %v", err)
	}
	moved := repository.lessons[first.Lesson.ID]
	if moved.ModuleID != "module-b" || moved.Position != 0 || moved.StableKey != first.Lesson.StableKey {
		t.Fatalf("move changed Lesson identity: %#v", moved)
	}
}

func TestNewLessonStableKeyIsOpaqueAndValid(t *testing.T) {
	first, err := newLessonStableKey()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newLessonStableKey()
	if err != nil {
		t.Fatal(err)
	}
	content := courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{}}
	if first == second || (LessonInput{DraftID: "draft", ModuleID: "module", StableKey: first, Title: "Lesson", Description: "Description", LearningObjectives: []string{"Objective"}, Content: content}).Validate() != nil || (LessonInput{DraftID: "draft", ModuleID: "module", StableKey: second, Title: "Lesson", Description: "Description", LearningObjectives: []string{"Objective"}, Content: content}).Validate() != nil {
		t.Fatalf("invalid generated Lesson keys %q %q", first, second)
	}
}

func TestLessonCreateInputRejectsInvalidAuthorMetadata(t *testing.T) {
	input := LessonCreateInput{DraftID: "draft", ModuleID: "module", Title: "Lesson", Description: "Description", LearningObjectives: []string{"Objective"}}
	if err := input.Validate(); err != nil {
		t.Fatal(err)
	}
	input.LearningObjectives = []string{""}
	if err := input.Validate(); err == nil {
		t.Fatalf("invalid objectives accepted: %v", err)
	}
}
