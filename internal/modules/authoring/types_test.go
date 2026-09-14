package authoring

import (
	"reflect"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

func validMetadata(t *testing.T) DraftMetadata {
	t.Helper()
	v, err := courses.ParseVersion("1.10.0")
	if err != nil {
		t.Fatal(err)
	}
	return DraftMetadata{CourseID: "course-id", IntendedVersion: v, SourceLanguage: "en", Title: "Event architecture", Description: "A draft course.", LearningObjectives: []string{"Explain events"}, Changelog: "Initial draft.", License: courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"}}
}

func TestDraftDomainValidation(t *testing.T) {
	m := validMetadata(t)
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	m.IntendedVersion = courses.Version{Major: -1}
	if err := m.Validate(); err == nil {
		t.Fatal("invalid intended version accepted")
	}
	if DraftStatus("PUBLISHED").Valid() || !DraftActive.Valid() || !DraftAbandoned.Valid() {
		t.Fatal("draft status confused with published lifecycle")
	}
	if MemberRole("REVIEWER").Valid() || !MemberAuthor.Valid() || !MemberMaintainer.Valid() {
		t.Fatal("invalid membership role")
	}
}

func TestDraftStructureValidation(t *testing.T) {
	module := ModuleInput{DraftID: "draft-id", StableKey: "event-basics", Title: "Basics", Position: 0}
	if err := module.Validate(); err != nil {
		t.Fatal(err)
	}
	module.StableKey = "Bad Key"
	if module.Validate() == nil {
		t.Fatal("invalid module key accepted")
	}
	module.StableKey = " event-basics "
	if module.Validate() == nil {
		t.Fatal("noncanonical module key accepted")
	}
	module.StableKey = "event-basics"
	module.Title = " "
	if module.Validate() == nil {
		t.Fatal("empty module title accepted")
	}
	module.Title = "Basics"
	module.Position = -1
	if module.Validate() == nil {
		t.Fatal("invalid module position accepted")
	}
	lesson := LessonInput{DraftID: "draft-id", ModuleID: "module-id", StableKey: "what-are-events", Title: "Events", Description: "An overview.", LearningObjectives: []string{"Explain an event"}, Position: 0, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{}}}
	if err := lesson.Validate(); err != nil {
		t.Fatal(err)
	}
	lesson.StableKey = "Bad Key"
	if lesson.Validate() == nil {
		t.Fatal("invalid lesson key accepted")
	}
	lesson.StableKey = " what-are-events "
	if lesson.Validate() == nil {
		t.Fatal("noncanonical lesson key accepted")
	}
	lesson.StableKey = "what-are-events"
	lesson.Title = " "
	if lesson.Validate() == nil {
		t.Fatal("empty lesson title accepted")
	}
	lesson.Title = "Events"
	invalid := 0
	lesson.EstimatedDurationMinutes = &invalid
	if lesson.Validate() == nil {
		t.Fatal("invalid lesson duration accepted")
	}
	lesson.EstimatedDurationMinutes = nil
	lesson.Content.SchemaVersion = 2
	if lesson.Validate() == nil {
		t.Fatal("invalid semantic content accepted")
	}
	if _, ok := reflect.TypeOf(DraftLesson{}).FieldByName("Difficulty"); ok {
		t.Fatal("lesson gained a difficulty field")
	}
}

func TestDraftModulePatchAndOrderValidation(t *testing.T) {
	current := ModuleInput{DraftID: "draft-id", StableKey: "stable-module", Title: "Original", Description: "Original description", Position: 2}
	title := "Updated"
	description := ""
	updated, err := (DraftModulePatch{Title: &title, Description: &description}).Apply(current)
	if err != nil || updated.Title != title || updated.Description != "" || updated.StableKey != current.StableKey || updated.Position != current.Position {
		t.Fatalf("module patch changed immutable structure: %#v err=%v", updated, err)
	}
	if _, err := (DraftModulePatch{}).Apply(current); err == nil {
		t.Fatal("empty module patch accepted")
	}
	blank := " "
	if _, err := (DraftModulePatch{Title: &blank}).Apply(current); err == nil {
		t.Fatal("blank module title accepted")
	}
	if err := ValidateModuleOrder([]ModuleID{"first", "second"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateModuleOrder([]ModuleID{"first", "first"}); err == nil {
		t.Fatal("duplicate module order accepted")
	}
	if err := ValidateModuleOrder(make([]ModuleID, MaxModulesPerDraft+1)); err == nil {
		t.Fatal("oversized module order accepted")
	}
}

func TestPrerequisiteKeys(t *testing.T) {
	if err := ValidatePrerequisiteKeys("current-lesson", []string{"first-lesson", "second-lesson"}); err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{{"current-lesson"}, {"other-lesson", "other-lesson"}, {"Bad Key"}, {" other-lesson "}} {
		if ValidatePrerequisiteKeys("current-lesson", keys) == nil {
			t.Fatalf("invalid prerequisites accepted: %v", keys)
		}
	}
}
