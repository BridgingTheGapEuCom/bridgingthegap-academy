package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// LessonMutationRepository is the focused structural/metadata contract for
// draft Lessons. Canonical content is deliberately absent until M3.2f.
type LessonMutationRepository interface {
	CreateLessonAtPosition(context.Context, DraftID, ModuleID, int64, LessonInput) (DraftLesson, CourseDraft, error)
	UpdateLessonMetadataForDraft(context.Context, DraftID, LessonID, int64, DraftLessonPatch) (DraftLesson, CourseDraft, error)
	ReorderLessonsForDraft(context.Context, DraftID, int64, []ModuleLessonOrder) (CourseDraft, error)
	ReplaceLessonPrerequisitesForDraft(context.Context, DraftID, LessonID, int64, []string) (DraftLesson, CourseDraft, error)
	DeleteLessonForDraft(context.Context, DraftID, LessonID, int64, int64) (CourseDraft, error)
}

type LessonMutationResult struct {
	Lesson DraftLesson
	Draft  CourseDraft
}

type LessonMutationService struct {
	repository LessonMutationRepository
	authorizer Authorizer
}

func NewLessonMutationService(repository LessonMutationRepository, authorizer Authorizer) *LessonMutationService {
	return &LessonMutationService{repository: repository, authorizer: authorizer}
}

func (s *LessonMutationService) CreateLesson(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, moduleID ModuleID, expectedDraftRevision int64, input LessonInput) (LessonMutationResult, error) {
	if expectedDraftRevision < 1 || input.DraftID != draftID || input.ModuleID != moduleID || input.Validate() != nil {
		return LessonMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return LessonMutationResult{}, err
	}
	lesson, draft, err := s.repository.CreateLessonAtPosition(ctx, draftID, moduleID, expectedDraftRevision, input)
	return LessonMutationResult{Lesson: lesson, Draft: draft}, err
}

func (s *LessonMutationService) UpdateLesson(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, lessonID LessonID, expectedLessonRevision int64, patch DraftLessonPatch) (LessonMutationResult, error) {
	if expectedLessonRevision < 1 || patch.Empty() {
		return LessonMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return LessonMutationResult{}, err
	}
	lesson, draft, err := s.repository.UpdateLessonMetadataForDraft(ctx, draftID, lessonID, expectedLessonRevision, patch)
	return LessonMutationResult{Lesson: lesson, Draft: draft}, err
}

func (s *LessonMutationService) ReorderLessons(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, expectedDraftRevision int64, order []ModuleLessonOrder) (CourseDraft, error) {
	if expectedDraftRevision < 1 || ValidateLessonOrder(order) != nil {
		return CourseDraft{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return CourseDraft{}, err
	}
	return s.repository.ReorderLessonsForDraft(ctx, draftID, expectedDraftRevision, order)
}

func (s *LessonMutationService) ReplacePrerequisites(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, lessonID LessonID, expectedLessonRevision int64, keys []string) (LessonMutationResult, error) {
	if expectedLessonRevision < 1 || ValidatePrerequisiteKeys("", keys) != nil {
		return LessonMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return LessonMutationResult{}, err
	}
	lesson, draft, err := s.repository.ReplaceLessonPrerequisitesForDraft(ctx, draftID, lessonID, expectedLessonRevision, keys)
	return LessonMutationResult{Lesson: lesson, Draft: draft}, err
}

func (s *LessonMutationService) DeleteLesson(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, lessonID LessonID, expectedDraftRevision, expectedLessonRevision int64) (CourseDraft, error) {
	if expectedDraftRevision < 1 || expectedLessonRevision < 1 {
		return CourseDraft{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return CourseDraft{}, err
	}
	return s.repository.DeleteLessonForDraft(ctx, draftID, lessonID, expectedDraftRevision, expectedLessonRevision)
}

func (s *LessonMutationService) authorize(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID) error {
	if s.repository == nil || s.authorizer == nil {
		return ErrAuthorizationUnavailable
	}
	err := s.authorizer.Authorize(ctx, actor, CapabilityStructureEdit, DraftResource(draftID))
	if errors.Is(err, ErrAuthorizationDenied) {
		return ErrNotFound
	}
	return err
}

// NewDraftLessonInput creates a content-less semantic draft lesson. M3.2e
// cannot accept or alter content; a later content-edit operation will replace
// this valid empty document through its own concurrency boundary.
func NewDraftLessonInput(draftID DraftID, moduleID ModuleID, stableKey, title, description string, objectives []string, durationMinutes *int, position int) LessonInput {
	return LessonInput{DraftID: draftID, ModuleID: moduleID, StableKey: stableKey, Title: title, Description: description, LearningObjectives: objectives, EstimatedDurationMinutes: durationMinutes, Position: position, Content: courses.LessonContent{SchemaVersion: courses.LessonContentSchemaVersion, Blocks: []courses.Block{}}}
}
