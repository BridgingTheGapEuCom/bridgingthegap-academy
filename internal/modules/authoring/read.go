package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// ModuleStructure is the ordered draft outline used by private Authoring
// reads. Lesson content is deliberately excluded from this projection.
type ModuleStructure struct {
	Module  DraftModule
	Lessons []LessonStructure
}

type LessonStructure struct {
	Lesson                      DraftLesson
	RecommendedPrerequisiteKeys []string
}

// ReadService loads mutable draft data only after the centralized, current
// membership check for the exact draft. Authorization denial is deliberately
// represented as ErrNotFound so private HTTP serving can hide draft existence.
type ReadService struct {
	repository ReadRepository
	authorizer Authorizer
}

func NewReadService(repository ReadRepository, authorizer Authorizer) *ReadService {
	return &ReadService{repository: repository, authorizer: authorizer}
}

func (s *ReadService) Draft(ctx context.Context, actor identity.AuthenticatedActor, id DraftID) (CourseDraft, error) {
	if err := s.authorize(ctx, actor, id); err != nil {
		return CourseDraft{}, err
	}
	return s.repository.GetDraft(ctx, id)
}

func (s *ReadService) Workspace(ctx context.Context, actor identity.AuthenticatedActor, id DraftID) (AuthoringWorkspace, error) {
	if err := s.authorize(ctx, actor, id); err != nil {
		return AuthoringWorkspace{}, err
	}
	return s.repository.GetWorkspace(ctx, id)
}

func (s *ReadService) Structure(ctx context.Context, actor identity.AuthenticatedActor, id DraftID) ([]ModuleStructure, error) {
	if err := s.authorize(ctx, actor, id); err != nil {
		return nil, err
	}
	modules, err := s.repository.ListModules(ctx, id)
	if err != nil {
		return nil, err
	}
	lessons, err := s.repository.ListLessonsForDraft(ctx, id)
	if err != nil {
		return nil, err
	}
	prerequisites, err := s.repository.ListPrerequisitesForDraft(ctx, id)
	if err != nil {
		return nil, err
	}
	prerequisiteKeys := make(map[LessonID][]string, len(lessons))
	for _, prerequisite := range prerequisites {
		prerequisiteKeys[prerequisite.LessonID] = append(prerequisiteKeys[prerequisite.LessonID], prerequisite.TargetStableKey)
	}
	byModule := make(map[ModuleID][]LessonStructure, len(modules))
	for _, lesson := range lessons {
		byModule[lesson.ModuleID] = append(byModule[lesson.ModuleID], LessonStructure{
			Lesson:                      lesson,
			RecommendedPrerequisiteKeys: prerequisiteKeys[lesson.ID],
		})
	}
	structure := make([]ModuleStructure, 0, len(modules))
	for _, module := range modules {
		structure = append(structure, ModuleStructure{Module: module, Lessons: byModule[module.ID]})
	}
	return structure, nil
}

func (s *ReadService) Lesson(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, lessonID LessonID) (DraftLesson, []Prerequisite, error) {
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return DraftLesson{}, nil, err
	}
	lesson, err := s.repository.GetLesson(ctx, lessonID)
	if err != nil {
		return DraftLesson{}, nil, err
	}
	if lesson.DraftID != draftID {
		return DraftLesson{}, nil, ErrNotFound
	}
	prerequisites, err := s.repository.ListPrerequisitesForDraft(ctx, draftID)
	if err != nil {
		return DraftLesson{}, nil, err
	}
	keys := make([]Prerequisite, 0)
	for _, prerequisite := range prerequisites {
		if prerequisite.LessonID == lessonID {
			keys = append(keys, prerequisite)
		}
	}
	return lesson, keys, nil
}

func (s *ReadService) authorize(ctx context.Context, actor identity.AuthenticatedActor, id DraftID) error {
	if s.repository == nil || s.authorizer == nil {
		return ErrAuthorizationUnavailable
	}
	err := s.authorizer.Authorize(ctx, actor, CapabilityRead, DraftResource(id))
	if errors.Is(err, ErrAuthorizationDenied) {
		return ErrNotFound
	}
	return err
}
