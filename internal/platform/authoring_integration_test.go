//go:build integration

package platform

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func draftFixture(t *testing.T, id courses.CourseID) authoring.DraftMetadata {
	t.Helper()
	version, err := courses.ParseVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return authoring.DraftMetadata{CourseID: id, IntendedVersion: version, SourceLanguage: "en", Title: "Draft course", Description: "Draft description.", LearningObjectives: []string{"Understand events"}, Changelog: "Initial draft.", License: courses.ContentLicense{Kind: courses.ContentLicenseAllRightsReserved, DisplayName: "All Rights Reserved"}}
}

func lessonFixture(draft authoring.DraftID, module authoring.ModuleID, key string, position int) authoring.LessonInput {
	return authoring.LessonInput{DraftID: draft, ModuleID: module, StableKey: key, Title: key, Description: "A lesson description.", LearningObjectives: []string{"Explain the concept"}, Position: position, Content: courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{}}}
}

func testAuthoringPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	cr := coursespostgres.New(pool)
	course, err := cr.CreateCourse(ctx, "authoring-integration-course")
	if err != nil {
		t.Fatal(err)
	}
	r := authoringpostgres.New(pool)
	creator := "11111111-1111-4111-8111-111111111111"
	missingCourse := draftFixture(t, courses.CourseID("33333333-3333-4333-8333-333333333333"))
	if _, _, err := r.CreateDraft(ctx, missingCourse, creator); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("draft targeting missing course: %v", err)
	}
	draft, workspace, err := r.CreateDraft(ctx, draftFixture(t, course.ID), creator)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Revision != 1 || draft.Status != authoring.DraftActive || workspace.DraftID != draft.ID {
		t.Fatal("draft/workspace creation mismatch")
	}
	loaded, err := r.GetDraft(ctx, draft.ID)
	if err != nil || loaded.Metadata.Title != "Draft course" {
		t.Fatalf("draft round trip: %v", err)
	}
	changedMetadata := loaded.Metadata
	changedMetadata.Title = "Edited draft course"
	editedDraft, err := r.UpdateDraftMetadata(ctx, draft.ID, loaded.Revision, changedMetadata)
	if err != nil || editedDraft.Revision != loaded.Revision+1 || editedDraft.Metadata.Title != "Edited draft course" {
		t.Fatalf("draft metadata revision: %v", err)
	}
	if _, err := r.UpdateDraftMetadata(ctx, draft.ID, loaded.Revision, loaded.Metadata); !errors.Is(err, authoring.ErrRevisionMismatch) {
		t.Fatalf("stale draft metadata overwrite: %v", err)
	}
	changedMetadata.CourseID = "22222222-2222-4222-8222-222222222222"
	if _, err := r.UpdateDraftMetadata(ctx, draft.ID, editedDraft.Revision, changedMetadata); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("draft course identity changed: %v", err)
	}
	gotWorkspace, err := r.GetWorkspace(ctx, draft.ID)
	if err != nil || gotWorkspace.ID != workspace.ID {
		t.Fatalf("workspace round trip: %v", err)
	}
	initialMembers, err := r.ListMembers(ctx, workspace.ID)
	if err != nil || len(initialMembers) != 1 || initialMembers[0].UserID != creator || initialMembers[0].Role != authoring.MemberMaintainer {
		t.Fatalf("creator membership: %v %#v", err, initialMembers)
	}
	collaborator := "22222222-2222-4222-8222-222222222222"
	member, err := r.AddMember(ctx, workspace.ID, collaborator, authoring.MemberAuthor)
	if err != nil || member.Role != authoring.MemberAuthor {
		t.Fatalf("member insert: %v", err)
	}
	if _, err := r.AddMember(ctx, workspace.ID, collaborator, authoring.MemberMaintainer); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("duplicate active member accepted: %v", err)
	}
	revoked, err := r.RevokeMember(ctx, workspace.ID, collaborator)
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("member revoke: %v", err)
	}
	if _, err := r.AddMember(ctx, workspace.ID, collaborator, authoring.MemberMaintainer); err != nil {
		t.Fatalf("member re-add: %v", err)
	}
	members, err := r.ListMembers(ctx, workspace.ID)
	if err != nil || len(members) != 3 {
		t.Fatalf("membership history: %v %#v", err, members)
	}

	advanced, err := r.CreateModule(ctx, authoring.ModuleInput{DraftID: draft.ID, StableKey: "advanced", Title: "Advanced", Position: 1})
	if err != nil {
		t.Fatal(err)
	}
	basics, err := r.CreateModule(ctx, authoring.ModuleInput{DraftID: draft.ID, StableKey: "basics", Title: "Basics", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	modules, err := r.ListModules(ctx, draft.ID)
	if err != nil || len(modules) != 2 || modules[0].ID != basics.ID {
		t.Fatalf("module order: %v %#v", err, modules)
	}
	if _, err := r.CreateModule(ctx, authoring.ModuleInput{DraftID: draft.ID, StableKey: "basics", Title: "Dup", Position: 2}); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("duplicate module key: %v", err)
	}
	if _, err := r.CreateModule(ctx, authoring.ModuleInput{DraftID: draft.ID, StableKey: "different", Title: "Dup", Position: 0}); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("duplicate module position: %v", err)
	}
	currentDraft, err := r.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := r.ReorderModules(ctx, draft.ID, currentDraft.Revision, []authoring.ModuleID{advanced.ID, basics.ID})
	if err != nil || reordered.Revision != currentDraft.Revision+1 {
		t.Fatalf("module reorder: %v", err)
	}
	if _, err := r.ReorderModules(ctx, draft.ID, currentDraft.Revision, []authoring.ModuleID{basics.ID, advanced.ID}); !errors.Is(err, authoring.ErrRevisionMismatch) {
		t.Fatalf("stale module reorder: %v", err)
	}
	modules, err = r.ListModules(ctx, draft.ID)
	if err != nil || modules[0].ID != advanced.ID {
		t.Fatalf("reordered modules: %v", err)
	}
	basics, err = r.GetModule(ctx, basics.ID)
	if err != nil {
		t.Fatal(err)
	}
	editedModule, err := r.UpdateModule(ctx, basics.ID, basics.Revision, "Edited basics", "")
	if err != nil || editedModule.Revision != basics.Revision+1 {
		t.Fatalf("module revision: %v", err)
	}
	if _, err := r.UpdateModule(ctx, basics.ID, basics.Revision, "Stale", ""); !errors.Is(err, authoring.ErrRevisionMismatch) {
		t.Fatalf("stale module update: %v", err)
	}
	basics = editedModule

	second, err := r.CreateLesson(ctx, lessonFixture(draft.ID, basics.ID, "second-lesson", 1))
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.CreateLesson(ctx, lessonFixture(draft.ID, basics.ID, "first-lesson", 0))
	if err != nil {
		t.Fatal(err)
	}
	lessons, err := r.ListLessons(ctx, basics.ID)
	if err != nil || len(lessons) != 2 || lessons[0].ID != first.ID {
		t.Fatalf("lesson ordering: %v", err)
	}
	if _, err := r.CreateLesson(ctx, lessonFixture(draft.ID, advanced.ID, "first-lesson", 0)); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("draft-wide key uniqueness: %v", err)
	}
	if _, err := r.CreateLesson(ctx, lessonFixture(draft.ID, basics.ID, "other-lesson", 1)); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("lesson position uniqueness: %v", err)
	}
	content := courses.LessonContent{SchemaVersion: 1, Blocks: []courses.Block{{Key: "intro", Type: courses.BlockDivider, Payload: courses.DividerBlockPayload{}}}}
	updated, err := r.UpdateLessonContent(ctx, first.ID, first.Revision, content)
	if err != nil || updated.Revision != first.Revision+1 || len(updated.Content.Blocks) != 1 {
		t.Fatalf("content round trip: %v", err)
	}
	if _, err := r.UpdateLessonContent(ctx, first.ID, first.Revision, content); !errors.Is(err, authoring.ErrRevisionMismatch) {
		t.Fatalf("stale content update: %v", err)
	}
	invalidContent := content
	invalidContent.SchemaVersion = 2
	if _, err := r.UpdateLessonContent(ctx, first.ID, updated.Revision, invalidContent); err == nil {
		t.Fatal("repository persisted unsupported content schema")
	}
	unchanged, err := r.GetLesson(ctx, first.ID)
	if err != nil || unchanged.Revision != updated.Revision {
		t.Fatalf("invalid content changed lesson: %v", err)
	}
	if _, err := r.ReplacePrerequisites(ctx, second.ID, second.Revision, []string{"first-lesson"}); err != nil {
		t.Fatal(err)
	}
	prereqs, err := r.ListPrerequisites(ctx, second.ID)
	if err != nil || len(prereqs) != 1 || prereqs[0].TargetStableKey != "first-lesson" {
		t.Fatalf("prerequisite round trip: %v", err)
	}
	if _, err := r.ReplacePrerequisites(ctx, updated.ID, updated.Revision, []string{"first-lesson"}); err == nil {
		t.Fatal("self prerequisite accepted")
	}

	other, _, err := r.CreateDraft(ctx, draftFixture(t, course.ID), creator)
	if err != nil {
		t.Fatal(err)
	}
	otherModule, err := r.CreateModule(ctx, authoring.ModuleInput{DraftID: other.ID, StableKey: "basics", Title: "Basics", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	otherLesson, err := r.CreateLesson(ctx, lessonFixture(other.ID, otherModule.ID, "first-lesson", 0))
	if err != nil || otherLesson.ID == first.ID {
		t.Fatalf("cross-draft key reuse: %v", err)
	}
	if _, err := r.CreateLesson(ctx, lessonFixture(draft.ID, otherModule.ID, "wrong-draft", 1)); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("lesson linked to module in another draft: %v", err)
	}
	if _, err := r.MoveLesson(ctx, first.ID, updated.Revision, otherModule.ID, 0); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("cross-draft move accepted: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO authoring.lesson_prerequisite(draft_id,lesson_id,prerequisite_lesson_id,position) VALUES($1,$2,$3,1)", draft.ID, second.ID, otherLesson.ID); err == nil {
		t.Fatal("cross-draft prerequisite accepted")
	}
	if _, err := pool.Exec(ctx, "INSERT INTO authoring.lesson_prerequisite(draft_id,lesson_id,prerequisite_lesson_id,position) VALUES($1,$2,$2,1)", draft.ID, second.ID); err == nil {
		t.Fatal("self prerequisite accepted by DB")
	}
	if _, err := pool.Exec(ctx, "UPDATE authoring.lesson SET content = '{\"schemaVersion\":1,\"blocks\":null}'::jsonb WHERE id=$1", first.ID); err == nil {
		t.Fatal("invalid top-level content accepted by DB")
	}

	// Two writers carry the same revision; exactly one may overwrite the row.
	barrier := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, title := range []string{"Writer A", "Writer B"} {
		wg.Add(1)
		go func(title string) {
			defer wg.Done()
			<-barrier
			_, e := r.UpdateLessonMetadata(ctx, first.ID, updated.Revision, title, "A lesson description.", []string{"Explain the concept"}, nil)
			results <- e
		}(title)
	}
	close(barrier)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, authoring.ErrRevisionMismatch) {
			conflict++
		} else {
			t.Fatalf("unexpected concurrent update: %v", e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("optimistic conflict result: success=%d conflict=%d", success, conflict)
	}
	first, err = r.GetLesson(ctx, first.ID)
	if err != nil || first.Revision != updated.Revision+1 {
		t.Fatalf("winning update lost: %v", err)
	}
	basics, err = r.GetModule(ctx, basics.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReorderLessons(ctx, basics.ID, basics.Revision, []authoring.LessonID{second.ID, first.ID}); err != nil {
		t.Fatalf("lesson reorder: %v", err)
	}
	lessons, err = r.ListLessons(ctx, basics.ID)
	if err != nil || lessons[0].ID != second.ID {
		t.Fatalf("reordered lessons: %v", err)
	}
	first, err = r.GetLesson(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.MoveLesson(ctx, first.ID, first.Revision, advanced.ID, 0); err != nil {
		t.Fatalf("move lesson: %v", err)
	}
	first, err = r.GetLesson(ctx, first.ID)
	if err != nil || first.ModuleID != advanced.ID || first.StableKey != "first-lesson" {
		t.Fatalf("move changed identity: %v", err)
	}
	if err := r.DeleteLesson(ctx, first.ID, first.Revision); err != nil {
		t.Fatalf("lesson delete: %v", err)
	}
	prereqs, err = r.ListPrerequisites(ctx, second.ID)
	if err != nil || len(prereqs) != 0 {
		t.Fatalf("deletion left dangling prerequisite: %v", err)
	}
	advanced, err = r.GetModule(ctx, advanced.ID)
	if err != nil {
		t.Fatal(err)
	}
	contained, err := r.CreateLesson(ctx, lessonFixture(draft.ID, advanced.ID, "contained-lesson", 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err = r.GetLesson(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReplacePrerequisites(ctx, second.ID, second.Revision, []string{"contained-lesson"}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteModule(ctx, advanced.ID, advanced.Revision); err != nil {
		t.Fatalf("module delete: %v", err)
	}
	if _, err := r.GetLesson(ctx, contained.ID); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("module deletion retained lesson: %v", err)
	}
	prereqs, err = r.ListPrerequisites(ctx, second.ID)
	if err != nil || len(prereqs) != 0 {
		t.Fatalf("module deletion left prerequisite: %v", err)
	}
	currentDraft, err = r.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	abandoned, err := r.AbandonDraft(ctx, draft.ID, currentDraft.Revision)
	if err != nil || abandoned.Status != authoring.DraftAbandoned {
		t.Fatalf("abandon draft: %v", err)
	}
	if _, err := r.CreateModule(ctx, authoring.ModuleInput{DraftID: draft.ID, StableKey: "blocked", Title: "Blocked", Position: 5}); !errors.Is(err, authoring.ErrInvalidState) {
		t.Fatalf("abandoned draft mutated: %v", err)
	}
	second, err = r.GetLesson(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateLessonContent(ctx, second.ID, second.Revision, second.Content); !errors.Is(err, authoring.ErrInvalidState) {
		t.Fatalf("abandoned draft lesson mutated: %v", err)
	}
	versions, err := cr.ListCourseVersions(ctx, course.ID)
	if err != nil || len(versions) != 0 {
		t.Fatalf("authoring created published course version: %v", err)
	}
}
