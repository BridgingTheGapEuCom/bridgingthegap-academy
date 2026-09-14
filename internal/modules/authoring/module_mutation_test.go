package authoring

import (
	"context"
	"errors"
	"testing"
)

type moduleMutationRepositoryFake struct {
	draft   CourseDraft
	modules map[ModuleID]DraftModule
}

func (r *moduleMutationRepositoryFake) CreateModuleAtPosition(_ context.Context, draftID DraftID, expected int64, input ModuleInput) (DraftModule, CourseDraft, error) {
	if expected != r.draft.Revision {
		return DraftModule{}, CourseDraft{}, ErrRevisionMismatch
	}
	if input.Position < 0 || input.Position > len(r.modules) {
		return DraftModule{}, CourseDraft{}, ErrInvalidStructure
	}
	for id, module := range r.modules {
		if module.Position >= input.Position {
			module.Position++
			module.Revision++
			r.modules[id] = module
		}
	}
	module := DraftModule{ID: ModuleID("new-module"), ModuleInput: input, Revision: 1}
	r.modules[module.ID] = module
	r.draft.Revision++
	return module, r.draft, nil
}

func (r *moduleMutationRepositoryFake) GetModule(_ context.Context, id ModuleID) (DraftModule, error) {
	module, found := r.modules[id]
	if !found {
		return DraftModule{}, ErrNotFound
	}
	return module, nil
}

func (r *moduleMutationRepositoryFake) UpdateModuleMetadata(_ context.Context, draftID DraftID, id ModuleID, expected int64, patch DraftModulePatch) (DraftModule, CourseDraft, error) {
	module, found := r.modules[id]
	if !found || module.DraftID != draftID {
		return DraftModule{}, CourseDraft{}, ErrNotFound
	}
	if module.Revision != expected {
		return DraftModule{}, CourseDraft{}, ErrRevisionMismatch
	}
	next, err := patch.Apply(module.ModuleInput)
	if err != nil {
		return DraftModule{}, CourseDraft{}, err
	}
	module.ModuleInput, module.Revision = next, module.Revision+1
	r.modules[id] = module
	r.draft.Revision++
	return module, r.draft, nil
}

func (r *moduleMutationRepositoryFake) ReorderModules(_ context.Context, draftID DraftID, expected int64, order []ModuleID) (CourseDraft, error) {
	if draftID != r.draft.ID {
		return CourseDraft{}, ErrNotFound
	}
	if expected != r.draft.Revision {
		return CourseDraft{}, ErrRevisionMismatch
	}
	if len(order) != len(r.modules) {
		return CourseDraft{}, ErrInvalidStructure
	}
	for position, id := range order {
		module, found := r.modules[id]
		if !found {
			return CourseDraft{}, ErrInvalidStructure
		}
		module.Position = position
		r.modules[id] = module
	}
	r.draft.Revision++
	return r.draft, nil
}

func (r *moduleMutationRepositoryFake) DeleteEmptyModule(_ context.Context, draftID DraftID, id ModuleID, expectedDraft, expectedModule int64) (CourseDraft, error) {
	module, found := r.modules[id]
	if !found || module.DraftID != draftID {
		return CourseDraft{}, ErrNotFound
	}
	if r.draft.Revision != expectedDraft || module.Revision != expectedModule {
		return CourseDraft{}, ErrRevisionMismatch
	}
	delete(r.modules, id)
	r.draft.Revision++
	return r.draft, nil
}

func TestModuleMutationServiceUsesScopedCapabilityAndRevisions(t *testing.T) {
	draft := mutationDraft()
	first := DraftModule{ID: "first", ModuleInput: ModuleInput{DraftID: draft.ID, StableKey: "first", Title: "First", Position: 0}, Revision: 2}
	repository := &moduleMutationRepositoryFake{draft: draft, modules: map[ModuleID]DraftModule{first.ID: first}}
	authorizer := &draftMutationAuthorizerFake{}
	service := NewModuleMutationService(repository, authorizer)
	created, err := service.CreateModule(context.Background(), resolvedActor(t), draft.ID, 3, ModuleInput{DraftID: draft.ID, StableKey: "second", Title: "Second", Position: 1})
	if err != nil || created.Module.StableKey != "second" || created.Draft.Revision != 4 || authorizer.capability != CapabilityStructureEdit || authorizer.resource != DraftResource(draft.ID) {
		t.Fatalf("module create did not use scoped structural capability: %#v err=%v", created, err)
	}
	title := "Renamed title only"
	updated, err := service.UpdateModule(context.Background(), resolvedActor(t), draft.ID, first.ID, 2, DraftModulePatch{Title: &title})
	if err != nil || updated.Module.Title != title || updated.Module.StableKey != "first" || updated.Module.Position != 0 || updated.Draft.Revision != 5 {
		t.Fatalf("module update changed structural identity: %#v err=%v", updated, err)
	}
	if _, err := service.UpdateModule(context.Background(), resolvedActor(t), draft.ID, first.ID, 2, DraftModulePatch{Title: &title}); !errors.Is(err, ErrRevisionMismatch) {
		t.Fatalf("stale module update = %v", err)
	}
	if _, err := service.ReorderModules(context.Background(), resolvedActor(t), draft.ID, 5, []ModuleID{"new-module", first.ID}); err != nil {
		t.Fatalf("full module reorder = %v", err)
	}
	if _, err := service.ReorderModules(context.Background(), resolvedActor(t), draft.ID, 6, []ModuleID{first.ID, first.ID}); !errors.Is(err, ErrInvalidStructure) {
		t.Fatalf("duplicate order = %v", err)
	}
	authorizer.err = ErrAuthorizationDenied
	if _, err := service.DeleteModule(context.Background(), resolvedActor(t), draft.ID, first.ID, 6, 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("denied structural mutation did not hide draft: %v", err)
	}
}

func TestModuleMutationServiceRejectsInvalidInputBeforeStorage(t *testing.T) {
	draft := mutationDraft()
	repository := &moduleMutationRepositoryFake{draft: draft, modules: map[ModuleID]DraftModule{}}
	service := NewModuleMutationService(repository, &draftMutationAuthorizerFake{})
	if _, err := service.CreateModule(context.Background(), resolvedActor(t), draft.ID, 3, ModuleInput{DraftID: draft.ID, StableKey: "Bad Key", Title: "Title", Position: 0}); !errors.Is(err, ErrInvalidStructure) {
		t.Fatalf("invalid stable key = %v", err)
	}
	if _, err := service.UpdateModule(context.Background(), resolvedActor(t), draft.ID, "missing", 1, DraftModulePatch{}); !errors.Is(err, ErrInvalidStructure) {
		t.Fatalf("empty patch = %v", err)
	}
}
