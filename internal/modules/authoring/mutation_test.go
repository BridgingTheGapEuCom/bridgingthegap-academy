package authoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
)

type draftMutationRepositoryFake struct {
	draft       CourseDraft
	getErr      error
	updateErr   error
	updateCalls int
}

func (r *draftMutationRepositoryFake) GetDraft(context.Context, DraftID) (CourseDraft, error) {
	if r.getErr != nil {
		return CourseDraft{}, r.getErr
	}
	return r.draft, nil
}

func (r *draftMutationRepositoryFake) UpdateDraftMetadata(_ context.Context, _ DraftID, expected int64, metadata DraftMetadata) (CourseDraft, error) {
	r.updateCalls++
	if r.updateErr != nil {
		return CourseDraft{}, r.updateErr
	}
	if expected != r.draft.Revision {
		return CourseDraft{}, ErrRevisionMismatch
	}
	r.draft.Metadata = metadata
	r.draft.Revision++
	r.draft.UpdatedAt = r.draft.UpdatedAt.Add(time.Second)
	return r.draft, nil
}

type draftMutationAuthorizerFake struct {
	err        error
	capability Capability
	resource   Resource
}

func (a *draftMutationAuthorizerFake) Authorize(_ context.Context, _ identity.AuthenticatedActor, capability Capability, resource Resource) error {
	a.capability, a.resource = capability, resource
	return a.err
}

func mutationDraft() CourseDraft {
	return CourseDraft{ID: testDraftA, Metadata: DraftMetadata{CourseID: "course", IntendedVersion: courses.Version{Major: 1}, SourceLanguage: "en", Title: "Original", Description: "Original description", LearningObjectives: []string{"Original objective"}, Changelog: "Initial draft", License: courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"}}, Status: DraftActive, Revision: 3, CreatedAt: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)}
}

func TestDraftMetadataPatchApply(t *testing.T) {
	title := "Updated"
	description := "Updated description"
	objectives := []string{"First", "Second"}
	version := courses.Version{Major: 1, Minor: 1}
	current := mutationDraft().Metadata
	next, err := (DraftMetadataPatch{Title: &title, Description: &description, LearningObjectives: &objectives, IntendedVersion: &version}).Apply(current)
	if err != nil {
		t.Fatal(err)
	}
	if next.Title != title || next.Description != description || next.IntendedVersion != version || len(next.LearningObjectives) != 2 || next.CourseID != current.CourseID || next.License != current.License {
		t.Fatalf("patch changed unexpected metadata: %#v", next)
	}
	objectives[0] = "mutated caller slice"
	if next.LearningObjectives[0] != "First" {
		t.Fatal("patch retained mutable request slice")
	}
	if _, err := (DraftMetadataPatch{}).Apply(current); err == nil {
		t.Fatal("empty metadata patch was accepted")
	}
	invalid := ""
	if _, err := (DraftMetadataPatch{Title: &invalid}).Apply(current); err == nil {
		t.Fatal("empty title patch was accepted")
	}
	invalidVersion := courses.Version{Major: -1}
	if _, err := (DraftMetadataPatch{IntendedVersion: &invalidVersion}).Apply(current); err == nil {
		t.Fatal("invalid intended version patch was accepted")
	}
	badObjectives := make([]string, 101)
	for i := range badObjectives {
		badObjectives[i] = "objective"
	}
	if _, err := (DraftMetadataPatch{LearningObjectives: &badObjectives}).Apply(current); err == nil {
		t.Fatal("oversized objectives patch was accepted")
	}
}

func TestDraftMutationServiceUsesCapabilityAndCompareAndSwap(t *testing.T) {
	repository := &draftMutationRepositoryFake{draft: mutationDraft()}
	authorizer := &draftMutationAuthorizerFake{}
	service := NewDraftMutationService(repository, authorizer)
	title := "Updated"
	updated, err := service.UpdateDraft(context.Background(), resolvedActor(t), testDraftA, 3, DraftMetadataPatch{Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	if authorizer.capability != CapabilityDraftEdit || authorizer.resource != DraftResource(testDraftA) || updated.Metadata.Title != title || updated.Revision != 4 || !updated.UpdatedAt.After(mutationDraft().UpdatedAt) {
		t.Fatalf("update did not use scoped capability or return committed revision: %#v", updated)
	}
	stale := "Stale"
	if _, err := service.UpdateDraft(context.Background(), resolvedActor(t), testDraftA, 3, DraftMetadataPatch{Title: &stale}); !errors.Is(err, ErrRevisionMismatch) {
		t.Fatalf("stale revision = %v", err)
	}
	if repository.draft.Metadata.Title != title || repository.draft.Revision != 4 {
		t.Fatalf("stale update overwrote persisted draft: %#v", repository.draft)
	}
	if _, err := service.UpdateDraft(context.Background(), resolvedActor(t), testDraftA, 4, DraftMetadataPatch{}); !errors.Is(err, ErrInvalidPatch) || repository.updateCalls != 2 {
		t.Fatalf("no-op patch reached storage or had wrong result: %v calls=%d", err, repository.updateCalls)
	}
	authorizer.err = ErrAuthorizationDenied
	if _, err := service.UpdateDraft(context.Background(), resolvedActor(t), testDraftA, 4, DraftMetadataPatch{Title: &title}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("authorization denial did not hide draft: %v", err)
	}
}
