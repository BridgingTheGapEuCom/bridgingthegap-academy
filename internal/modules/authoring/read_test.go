package authoring

import (
	"context"
	"errors"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

type readRepositoryFake struct {
	draft         CourseDraft
	workspace     AuthoringWorkspace
	modules       []DraftModule
	lessons       []DraftLesson
	prerequisites []Prerequisite
	err           error
}

func (r readRepositoryFake) GetDraft(context.Context, DraftID) (CourseDraft, error) {
	return r.draft, r.err
}
func (r readRepositoryFake) GetWorkspace(context.Context, DraftID) (AuthoringWorkspace, error) {
	return r.workspace, r.err
}
func (r readRepositoryFake) ReadStructure(_ context.Context, _ DraftID) ([]ModuleStructure, error) {
	result := make([]ModuleStructure, 0, len(r.modules))
	for _, module := range r.modules {
		item := ModuleStructure{Module: module}
		for _, lesson := range r.lessons {
			if lesson.ModuleID != module.ID {
				continue
			}
			keys := []string{}
			for _, p := range r.prerequisites {
				if p.LessonID == lesson.ID {
					keys = append(keys, p.TargetStableKey)
				}
			}
			item.Lessons = append(item.Lessons, LessonStructure{Lesson: lesson, RecommendedPrerequisiteKeys: keys})
		}
		result = append(result, item)
	}
	return result, r.err
}
func (r readRepositoryFake) ReadLesson(_ context.Context, draftID DraftID, id LessonID) (DraftLesson, []Prerequisite, error) {
	for _, lesson := range r.lessons {
		if lesson.ID == id && lesson.DraftID == draftID {
			return lesson, r.prerequisites, r.err
		}
	}
	return DraftLesson{}, nil, ErrNotFound
}

type readAuthorizerFake struct{ err error }

func (a readAuthorizerFake) Authorize(context.Context, identity.AuthenticatedActor, Capability, Resource) error {
	return a.err
}

func TestReadServiceHidesDeniedResourcesAndPreservesStructureOrder(t *testing.T) {
	draftID := DraftID("11111111-1111-4111-8111-111111111111")
	moduleA := DraftModule{ID: "module-a", ModuleInput: ModuleInput{DraftID: draftID, StableKey: "first-module", Title: "First", Position: 0}, Revision: 3}
	moduleB := DraftModule{ID: "module-b", ModuleInput: ModuleInput{DraftID: draftID, StableKey: "second-module", Title: "Second", Position: 1}, Revision: 4}
	content := courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{}}
	lessonA := DraftLesson{ID: "lesson-a", LessonInput: LessonInput{DraftID: draftID, ModuleID: moduleA.ID, StableKey: "first-lesson", Title: "First lesson", Description: "Description", LearningObjectives: []string{"Understand"}, Position: 0, Content: content}, Revision: 5}
	lessonB := DraftLesson{ID: "lesson-b", LessonInput: LessonInput{DraftID: draftID, ModuleID: moduleB.ID, StableKey: "second-lesson", Title: "Second lesson", Description: "Description", LearningObjectives: []string{"Apply"}, Position: 0, Content: content}, Revision: 6}
	repository := readRepositoryFake{modules: []DraftModule{moduleA, moduleB}, lessons: []DraftLesson{lessonA, lessonB}, prerequisites: []Prerequisite{{LessonID: lessonB.ID, TargetStableKey: lessonA.StableKey, Position: 0}}}
	service := NewReadService(repository, readAuthorizerFake{})
	structure, err := service.Structure(context.Background(), identity.AuthenticatedActor{}, draftID)
	if err != nil {
		t.Fatal(err)
	}
	if len(structure) != 2 || structure[0].Module.ID != moduleA.ID || structure[1].Lessons[0].Lesson.ID != lessonB.ID || structure[1].Lessons[0].RecommendedPrerequisiteKeys[0] != lessonA.StableKey {
		t.Fatalf("ordered structure or prerequisite keys were altered: %#v", structure)
	}

	denied := NewReadService(repository, readAuthorizerFake{err: ErrAuthorizationDenied})
	if _, err := denied.Structure(context.Background(), identity.AuthenticatedActor{}, draftID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("denied read = %v, want hidden not found", err)
	}
}

func TestReadServiceRejectsCrossDraftLessonAndPreservesFailures(t *testing.T) {
	draftA := DraftID("11111111-1111-4111-8111-111111111111")
	draftB := DraftID("22222222-2222-4222-8222-222222222222")
	lesson := DraftLesson{ID: "lesson-b", LessonInput: LessonInput{DraftID: draftB, ModuleID: "module-b", StableKey: "other-lesson", Title: "Other", Description: "Description", LearningObjectives: []string{"Apply"}, Content: courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{}}}}
	service := NewReadService(readRepositoryFake{lessons: []DraftLesson{lesson}}, readAuthorizerFake{})
	if _, _, err := service.Lesson(context.Background(), identity.AuthenticatedActor{}, draftA, lesson.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-draft lesson = %v, want not found", err)
	}

	unavailable := errors.New("database unavailable")
	service = NewReadService(readRepositoryFake{}, readAuthorizerFake{err: unavailable})
	if _, err := service.Draft(context.Background(), identity.AuthenticatedActor{}, draftA); !errors.Is(err, unavailable) {
		t.Fatalf("authorization outage = %v, want propagated error", err)
	}
}
