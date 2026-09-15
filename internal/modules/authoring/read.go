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
	return s.repository.ReadStructure(ctx, id)
}

func (s *ReadService) Lesson(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, lessonID LessonID) (DraftLesson, []Prerequisite, error) {
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return DraftLesson{}, nil, err
	}
	return s.repository.ReadLesson(ctx, draftID, lessonID)
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
