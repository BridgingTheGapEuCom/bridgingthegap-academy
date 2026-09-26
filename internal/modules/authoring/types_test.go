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

func TestDraftLessonPatchAndFullOrderValidation(t *testing.T) {
	duration := 20
	current := LessonInput{DraftID: "draft-id", ModuleID: "module-id", StableKey: "stable-lesson", Title: "Original", Description: "Description", LearningObjectives: []string{"Explain"}, EstimatedDurationMinutes: &duration, Position: 1, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{}}}
	title := "Updated"
	objectives := []string{"First", "Second"}
	next, err := (DraftLessonPatch{Title: &title, LearningObjectives: &objectives, EstimatedDurationSet: true}).Apply(current)
	if err != nil || next.Title != title || len(next.LearningObjectives) != 2 || next.EstimatedDurationMinutes != nil || next.StableKey != current.StableKey || next.ModuleID != current.ModuleID || next.Position != current.Position || !reflect.DeepEqual(next.Content, current.Content) {
		t.Fatalf("lesson patch changed structural/content identity: %#v err=%v", next, err)
	}
	if _, err := (DraftLessonPatch{}).Apply(current); err == nil {
		t.Fatal("empty lesson patch accepted")
	}
	invalidDuration := 0
	if _, err := (DraftLessonPatch{EstimatedDurationSet: true, EstimatedDurationMinutes: &invalidDuration}).Apply(current); err == nil {
		t.Fatal("invalid duration patch accepted")
	}
	valid := []ModuleLessonOrder{{ModuleID: "module-a", LessonIDs: []LessonID{"lesson-a", "lesson-b"}}, {ModuleID: "module-b", LessonIDs: []LessonID{}}}
	if err := ValidateLessonOrder(valid); err != nil {
		t.Fatal(err)
	}
	for _, order := range [][]ModuleLessonOrder{
		{{ModuleID: "module-a"}, {ModuleID: "module-a"}},
		{{ModuleID: "module-a", LessonIDs: []LessonID{"lesson-a", "lesson-a"}}},
		{{ModuleID: "", LessonIDs: []LessonID{"lesson-a"}}},
	} {
		if ValidateLessonOrder(order) == nil {
			t.Fatalf("invalid lesson order accepted: %#v", order)
		}
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
