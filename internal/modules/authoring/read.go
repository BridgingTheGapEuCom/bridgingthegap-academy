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

// Drafts returns only the mutable workspaces to which this trusted actor has a
// current membership. Listing uses the membership relation directly rather
// than treating a global role or a client-provided claim as access.
func (s *ReadService) Drafts(ctx context.Context, actor identity.AuthenticatedActor) ([]DraftSummary, error) {
	if s.repository == nil || actor.UserID() == "" || actor.SessionID() == "" {
		return nil, ErrAuthorizationUnavailable
	}
	return s.repository.ListAccessibleDrafts(ctx, string(actor.UserID()))
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

// Members returns the currently active, opaque workspace memberships for one
// draft. Authorization happens before the repository read so this projection
// cannot expose a draft's collaborators to an unrelated workspace member.
func (s *ReadService) Members(ctx context.Context, actor identity.AuthenticatedActor, id DraftID) ([]WorkspaceMember, error) {
	if err := s.authorize(ctx, actor, id); err != nil {
		return nil, err
	}
	return s.repository.ActiveMembers(ctx, id)
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
