package authoring

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

func reviewSnapshotFixture(t *testing.T) (CourseDraft, []DraftModule, []DraftLesson, []Prerequisite) {
	t.Helper()
	version, err := courses.ParseVersion("2.3.0")
	if err != nil {
		t.Fatal(err)
	}
	draft := CourseDraft{
		ID:       "10000000-0000-4000-8000-000000000001",
		Revision: 7,
		Metadata: DraftMetadata{
			CourseID:           "20000000-0000-4000-8000-000000000001",
			IntendedVersion:    version,
			SourceLanguage:     "en",
			Title:              "Review fixture",
			Description:        "A complete review fixture.",
			LearningObjectives: []string{"Explain frozen reviews"},
			Changelog:          "Ready for review.",
			License:            courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"},
		},
	}
	modules := []DraftModule{
		{ID: "30000000-0000-4000-8000-000000000001", ModuleInput: ModuleInput{DraftID: draft.ID, StableKey: "foundations", Title: "Foundations", Position: 0}},
		{ID: "30000000-0000-4000-8000-000000000002", ModuleInput: ModuleInput{DraftID: draft.ID, StableKey: "practice", Title: "Practice", Position: 1}},
	}
	content := courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "separator", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}}
	lessons := []DraftLesson{
		{ID: "40000000-0000-4000-8000-000000000001", LessonInput: LessonInput{DraftID: draft.ID, ModuleID: modules[0].ID, StableKey: "introduction", Title: "Introduction", Description: "Introduce the review model.", LearningObjectives: []string{"Describe a review cycle"}, Position: 0, Content: content}},
		{ID: "40000000-0000-4000-8000-000000000002", LessonInput: LessonInput{DraftID: draft.ID, ModuleID: modules[1].ID, StableKey: "exercise", Title: "Exercise", Description: "Apply the review model.", LearningObjectives: []string{"Apply a review cycle"}, Position: 0, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{}}}},
	}
	prerequisites := []Prerequisite{{LessonID: lessons[1].ID, TargetLessonID: lessons[0].ID, TargetStableKey: lessons[0].StableKey, Position: 0}}
	return draft, modules, lessons, prerequisites
}

func TestReviewSnapshotFreezesCanonicalDraftState(t *testing.T) {
	draft, modules, lessons, prerequisites := reviewSnapshotFixture(t)
	snapshot, err := NewReviewSnapshot(draft, modules, lessons, prerequisites)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Draft.Revision != 7 || snapshot.Draft.IntendedVersion != "2.3.0" || len(snapshot.Modules) != 2 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if got := snapshot.Modules[1].Lessons[0].PrerequisiteStableKeys; len(got) != 1 || got[0] != "introduction" {
		t.Fatalf("prerequisite order lost: %#v", got)
	}

	// Mutating every source layer after capture must not rewrite the review.
	draft.Metadata.Title = "Changed later"
	modules[0].Title = "Changed later"
	lessons[0].LearningObjectives[0] = "Changed later"
	lessons[0].Content.Blocks[0].Key = "changed-later"
	prerequisites[0].TargetStableKey = "changed-later"
	if snapshot.Draft.Title != "Review fixture" || snapshot.Modules[0].Title != "Foundations" || snapshot.Modules[0].Lessons[0].Objectives[0] != "Describe a review cycle" || snapshot.Modules[0].Lessons[0].Content.Blocks[0].Key != "separator" || snapshot.Modules[1].Lessons[0].PrerequisiteStableKeys[0] != "introduction" {
		t.Fatal("snapshot retained mutable source aliases")
	}

	encoded, err := MarshalReviewSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "editor") || strings.Contains(string(encoded), "tiptap") || !json.Valid(encoded) {
		t.Fatalf("snapshot contains non-canonical state: %s", encoded)
	}
}

func TestReviewSnapshotValidationRejectsBrokenFrozenState(t *testing.T) {
	draft, modules, lessons, prerequisites := reviewSnapshotFixture(t)
	snapshot, err := NewReviewSnapshot(draft, modules, lessons, prerequisites)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*ReviewSnapshot){
		"schema":       func(s *ReviewSnapshot) { s.SchemaVersion = 2 },
		"module order": func(s *ReviewSnapshot) { s.Modules[0].Position = 1 },
		"lesson id":    func(s *ReviewSnapshot) { s.Modules[0].Lessons[0].ID = "" },
		"unknown prereq": func(s *ReviewSnapshot) {
			s.Modules[1].Lessons[0].PrerequisiteStableKeys = []string{"missing-lesson"}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := snapshot
			encoded, _ := json.Marshal(snapshot)
			if err := json.Unmarshal(encoded, &candidate); err != nil {
				t.Fatal(err)
			}
			mutate(&candidate)
			if !errors.Is(candidate.Validate(), ErrReviewSnapshot) {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}

func TestReviewCycleDecisionTransitions(t *testing.T) {
	cycle := ReviewCycle{Status: ReviewInReview, Revision: 1, DraftRevision: 9}
	if err := cycle.CanDecide(1); err != nil {
		t.Fatalf("valid decision rejected: %v", err)
	}
	if !errors.Is(cycle.CanDecide(2), ErrReviewStale) {
		t.Fatal("stale review decision accepted")
	}
	cycle.Status = ReviewApproved
	if !errors.Is(cycle.CanDecide(1), ErrReviewInvalidState) {
		t.Fatal("approved review was not terminal")
	}
	if cycle.DraftRevision != 9 {
		t.Fatal("decision changed frozen revision")
	}
}
