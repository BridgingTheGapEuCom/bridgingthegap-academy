package authoring

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

// ModuleMutationRepository is the narrow structural-writing contract. It
// cannot mutate lesson data, membership, or published CourseVersion rows.
type ModuleMutationRepository interface {
	CreateModuleAtPosition(context.Context, DraftID, int64, ModuleInput) (DraftModule, CourseDraft, error)
	GetModule(context.Context, ModuleID) (DraftModule, error)
	UpdateModuleMetadata(context.Context, DraftID, ModuleID, int64, DraftModulePatch) (DraftModule, CourseDraft, error)
	ReorderModules(context.Context, DraftID, int64, []ModuleID) (CourseDraft, error)
	DeleteEmptyModule(context.Context, DraftID, ModuleID, int64, int64) (CourseDraft, error)
}

// ModuleMutationResult returns the exact committed module state and aggregate
// draft revision needed by a later authoring client.
type ModuleMutationResult struct {
	Module DraftModule
	Draft  CourseDraft
}

// ModuleMutationService enforces current, resource-scoped authorization before
// every mutable structural operation. Denied drafts remain hidden to private
// HTTP clients.
type ModuleMutationService struct {
	repository ModuleMutationRepository
	authorizer Authorizer
}

func NewModuleMutationService(repository ModuleMutationRepository, authorizer Authorizer) *ModuleMutationService {
	return &ModuleMutationService{repository: repository, authorizer: authorizer}
}

func (s *ModuleMutationService) CreateModule(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, expectedDraftRevision int64, input ModuleInput) (ModuleMutationResult, error) {
	if expectedDraftRevision < 1 || input.DraftID != draftID || input.Validate() != nil {
		return ModuleMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return ModuleMutationResult{}, err
	}
	module, draft, err := s.repository.CreateModuleAtPosition(ctx, draftID, expectedDraftRevision, input)
	return ModuleMutationResult{Module: module, Draft: draft}, err
}

func (s *ModuleMutationService) UpdateModule(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, moduleID ModuleID, expectedModuleRevision int64, patch DraftModulePatch) (ModuleMutationResult, error) {
	if expectedModuleRevision < 1 || patch.Empty() {
		return ModuleMutationResult{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return ModuleMutationResult{}, err
	}
	module, draft, err := s.repository.UpdateModuleMetadata(ctx, draftID, moduleID, expectedModuleRevision, patch)
	return ModuleMutationResult{Module: module, Draft: draft}, err
}

func (s *ModuleMutationService) ReorderModules(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, expectedDraftRevision int64, order []ModuleID) (CourseDraft, error) {
	if expectedDraftRevision < 1 || ValidateModuleOrder(order) != nil {
		return CourseDraft{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return CourseDraft{}, err
	}
	return s.repository.ReorderModules(ctx, draftID, expectedDraftRevision, order)
}

func (s *ModuleMutationService) DeleteModule(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID, moduleID ModuleID, expectedDraftRevision, expectedModuleRevision int64) (CourseDraft, error) {
	if expectedDraftRevision < 1 || expectedModuleRevision < 1 {
		return CourseDraft{}, ErrInvalidStructure
	}
	if err := s.authorize(ctx, actor, draftID); err != nil {
		return CourseDraft{}, err
	}
	return s.repository.DeleteEmptyModule(ctx, draftID, moduleID, expectedDraftRevision, expectedModuleRevision)
}

func (s *ModuleMutationService) authorize(ctx context.Context, actor identity.AuthenticatedActor, draftID DraftID) error {
	if s.repository == nil || s.authorizer == nil {
		return ErrAuthorizationUnavailable
	}
	err := s.authorizer.Authorize(ctx, actor, CapabilityStructureEdit, DraftResource(draftID))
	if errors.Is(err, ErrAuthorizationDenied) {
		return ErrNotFound
	}
	return err
}
