package authoring

import (
	"context"
	"errors"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type lessonContentMutationRepositoryFake struct {
	draft  CourseDraft
	lesson DraftLesson
}

func (r *lessonContentMutationRepositoryFake) ReplaceLessonContentForDraft(_ context.Context, draftID DraftID, lessonID LessonID, expected int64, content courses.LessonContent) (DraftLesson, CourseDraft, error) {
	if draftID != r.draft.ID || lessonID != r.lesson.ID {
		return DraftLesson{}, CourseDraft{}, ErrNotFound
	}
	if expected != r.lesson.Revision {
		return DraftLesson{}, CourseDraft{}, ErrRevisionMismatch
	}
	r.lesson.Content = content
	r.lesson.Revision++
	r.draft.Revision++
	return r.lesson, r.draft, nil
}

func TestLessonContentMutationServiceUsesContentCapabilityAndCAS(t *testing.T) {
	draft := mutationDraft()
	lesson := DraftLesson{ID: "lesson", LessonInput: LessonInput{DraftID: draft.ID, ModuleID: "module", StableKey: "lesson", Title: "Lesson", Description: "Description", LearningObjectives: []string{"Understand"}, Content: courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{}}}, Revision: 3}
	repository := &lessonContentMutationRepositoryFake{draft: draft, lesson: lesson}
	authorizer := &draftMutationAuthorizerFake{}
	service := NewLessonContentMutationService(repository, authorizer)
	content := courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{{Key: "divider", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}}
	result, err := service.ReplaceContent(context.Background(), resolvedActor(t), draft.ID, lesson.ID, 3, content)
	if err != nil || result.Lesson.Revision != 4 || result.Draft.Revision != 4 || result.Lesson.Content.Blocks[0].Key != "divider" || authorizer.capability != CapabilityContentEdit || authorizer.resource != DraftResource(draft.ID) {
		t.Fatalf("content replacement did not use scoped content capability and revisions: %#v err=%v", result, err)
	}
	if _, err := service.ReplaceContent(context.Background(), resolvedActor(t), draft.ID, lesson.ID, 3, content); !errors.Is(err, ErrRevisionMismatch) {
		t.Fatalf("stale content replacement = %v", err)
	}
	invalid := content
	invalid.SchemaVersion = 2
	if _, err := service.ReplaceContent(context.Background(), resolvedActor(t), draft.ID, lesson.ID, 4, invalid); !errors.Is(err, ErrInvalidStructure) {
		t.Fatalf("invalid canonical content = %v", err)
	}
	authorizer.err = ErrAuthorizationDenied
	if _, err := service.ReplaceContent(context.Background(), resolvedActor(t), draft.ID, lesson.ID, 4, content); !errors.Is(err, ErrNotFound) {
		t.Fatalf("denied content mutation did not hide draft: %v", err)
	}
}
