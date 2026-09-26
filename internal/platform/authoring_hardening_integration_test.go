//go:build integration

package platform

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	authoringpostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/postgres"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAuthoringHardening(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	r := authoringpostgres.New(pool)
	course, err := coursespostgres.New(pool).CreateCourse(ctx, "authoring-hardening")
	if err != nil {
		t.Fatal(err)
	}
	creator := "11111111-1111-4111-8111-111111111111"
	collaborator := "22222222-2222-4222-8222-222222222222"
	makeDraft := func() authoring.CourseDraft {
		draft, _, err := r.CreateDraft(ctx, draftFixture(t, course.ID), creator)
		if err != nil {
			t.Fatal(err)
		}
		return draft
	}
	draft := makeDraft()
	module, draft, err := r.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "module-one", Title: "Module", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	target, draft, err := r.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, lessonFixture(draft.ID, module.ID, "target-lesson", 0))
	if err != nil {
		t.Fatal(err)
	}
	loadedModule, err := r.GetModule(ctx, module.ID)
	if err != nil || loadedModule.Revision != module.Revision+1 {
		t.Fatalf("lesson creation did not advance owning module: %v %#v", err, loadedModule)
	}
	source, draft, err := r.CreateLessonAtPosition(ctx, draft.ID, module.ID, draft.Revision, lessonFixture(draft.ID, module.ID, "source-lesson", 1))
	if err != nil {
		t.Fatal(err)
	}
	source, draft, err = r.ReplaceLessonPrerequisitesForDraft(ctx, draft.ID, source.ID, source.Revision, []string{target.StableKey})
	if err != nil {
		t.Fatal(err)
	}

	// Invalid replacements must retain both the previous relations and revisions.
	if _, _, err := r.ReplaceLessonPrerequisitesForDraft(ctx, draft.ID, source.ID, source.Revision, []string{"missing-target"}); !errors.Is(err, authoring.ErrConflict) {
		t.Fatalf("missing prerequisite = %v", err)
	}
	unchanged, err := r.GetLesson(ctx, source.ID)
	if err != nil || unchanged.Revision != source.Revision {
		t.Fatalf("failed prerequisites changed revision: %v", err)
	}
	relations, err := r.ListPrerequisites(ctx, source.ID)
	if err != nil || len(relations) != 1 || relations[0].TargetLessonID != target.ID {
		t.Fatalf("failed prerequisites changed relations: %v %#v", err, relations)
	}

	beforeModule, err := r.GetModule(ctx, module.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A stale delete must roll back incoming-reference cleanup and compaction.
	if _, err := r.DeleteLessonForDraft(ctx, draft.ID, target.ID, draft.Revision, target.Revision+1); !errors.Is(err, authoring.ErrRevisionMismatch) {
		t.Fatalf("stale delete = %v", err)
	}
	afterFailure, err := r.GetLesson(ctx, source.ID)
	if err != nil || afterFailure.Revision != source.Revision || afterFailure.Position != 1 {
		t.Fatalf("stale delete partially changed source: %v %#v", err, afterFailure)
	}
	relations, err = r.ListPrerequisites(ctx, source.ID)
	if err != nil || len(relations) != 1 {
		t.Fatalf("stale delete removed incoming relations: %v", err)
	}
	draft, err = r.DeleteLessonForDraft(ctx, draft.ID, target.ID, draft.Revision, target.Revision)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := r.GetLesson(ctx, source.ID)
	if err != nil || changed.Revision != source.Revision+1 || changed.Position != 0 {
		t.Fatalf("deletion must advance affected lesson once and compact: %v %#v", err, changed)
	}
	afterModule, err := r.GetModule(ctx, module.ID)
	if err != nil || afterModule.Revision != beforeModule.Revision+1 {
		t.Fatalf("delete did not advance owning module: %v", err)
	}
	relations, err = r.ListPrerequisites(ctx, source.ID)
	if err != nil || len(relations) != 0 {
		t.Fatalf("deleted target remains referenced: %v", err)
	}

	other := makeDraft()
	foreignModule, other, err := r.CreateModuleAtPosition(ctx, other.ID, other.Revision, authoring.ModuleInput{DraftID: other.ID, StableKey: "foreign-module", Title: "Foreign", Position: 0})
	if err != nil {
		t.Fatal(err)
	}
	foreignLesson, _, err := r.CreateLessonAtPosition(ctx, other.ID, foreignModule.ID, other.Revision, lessonFixture(other.ID, foreignModule.ID, "foreign-lesson", 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		run  func() error
	}{
		{"foreign module reorder", func() error {
			_, e := r.ReorderModules(ctx, draft.ID, draft.Revision, []authoring.ModuleID{foreignModule.ID})
			return e
		}},
		{"foreign module create with stale revision", func() error {
			_, _, e := r.CreateLessonAtPosition(ctx, draft.ID, foreignModule.ID, 1, lessonFixture(draft.ID, foreignModule.ID, "invalid-lesson", 0))
			return e
		}},
		{"foreign module delete with stale revision", func() error { _, e := r.DeleteEmptyModule(ctx, draft.ID, foreignModule.ID, 1, 1); return e }},
		{"foreign lesson delete with stale revision", func() error { _, e := r.DeleteLessonForDraft(ctx, draft.ID, foreignLesson.ID, 1, 1); return e }},
		{"foreign layout", func() error {
			_, e := r.ReorderLessonsForDraft(ctx, draft.ID, draft.Revision, []authoring.ModuleLessonOrder{{ModuleID: module.ID, LessonIDs: []authoring.LessonID{foreignLesson.ID}}})
			return e
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if e := test.run(); !errors.Is(e, authoring.ErrNotFound) {
				t.Fatalf("foreign resource = %v", e)
			}
		})
	}
	stored, err := r.GetDraft(ctx, draft.ID)
	if err != nil || stored.Revision != draft.Revision {
		t.Fatalf("failed structure mutation changed draft: %v", err)
	}

	for _, invalid := range []string{`{}`, `{"schemaVersion":1}`, `{"blocks":[]}`, `{"schemaVersion":"1","blocks":[]}`, `{"schemaVersion":null,"blocks":[]}`} {
		if _, err := pool.Exec(ctx, "UPDATE authoring.lesson SET content=$1::jsonb WHERE id=$2", invalid, source.ID); err == nil {
			t.Fatalf("malformed top-level content accepted: %s", invalid)
		}
	}

	// The outline must not load/validate content or expose null prerequisite arrays.
	if _, err := pool.Exec(ctx, `UPDATE authoring.lesson SET content='{"schemaVersion":1,"blocks":[{"type":"UNKNOWN"}]}'::jsonb WHERE id=$1`, source.ID); err != nil {
		t.Fatal(err)
	}
	outline, err := r.ReadStructure(ctx, draft.ID)
	if err != nil || len(outline) != 1 || len(outline[0].Lessons) != 1 || outline[0].Lessons[0].RecommendedPrerequisiteKeys == nil {
		t.Fatalf("metadata-only outline failed: %v %#v", err, outline)
	}
	if _, _, err := r.ReadLesson(ctx, other.ID, source.ID); !errors.Is(err, authoring.ErrNotFound) {
		t.Fatalf("foreign invalid content leaked through decoding: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE authoring.lesson SET content='{"schemaVersion":1,"blocks":[]}'::jsonb WHERE id=$1`, source.ID); err != nil {
		t.Fatal(err)
	}

	// Metadata and prerequisite writes share the Lesson CAS guard.
	freshSource, err := r.GetLesson(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	title := "Concurrent metadata"
	runAuthoringRace(t, []func() error{
		func() error {
			_, _, e := r.UpdateLessonMetadataForDraft(ctx, draft.ID, freshSource.ID, freshSource.Revision, authoring.DraftLessonPatch{Title: &title})
			return e
		},
		func() error {
			_, _, e := r.ReplaceLessonPrerequisitesForDraft(ctx, draft.ID, freshSource.ID, freshSource.Revision, []string{})
			return e
		},
	})
	finalSource, err := r.GetLesson(ctx, source.ID)
	if err != nil || finalSource.Revision != freshSource.Revision+1 {
		t.Fatalf("lesson CAS lost update: %v", err)
	}

	// Pause after reading Modules; a committed move must not alter the reader's snapshot.
	testAuthoringReadSnapshot(t, ctx, pool, r, draft.ID, module.ID, source.ID)
	// A deletion racing prerequisite replacement cannot resurrect a deleted reference.
	current, err := r.GetDraft(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	prerequisite, current, err := r.CreateLessonAtPosition(ctx, current.ID, module.ID, current.Revision, lessonFixture(current.ID, module.ID, "race-prerequisite", 1))
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := r.GetLesson(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, current, err = r.ReplaceLessonPrerequisitesForDraft(ctx, current.ID, refreshed.ID, refreshed.Revision, []string{prerequisite.StableKey})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err = r.GetLesson(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	runAuthoringRace(t, []func() error{
		func() error {
			_, e := r.DeleteLessonForDraft(ctx, current.ID, prerequisite.ID, current.Revision, prerequisite.Revision)
			return e
		},
		func() error {
			_, _, e := r.ReplaceLessonPrerequisitesForDraft(ctx, current.ID, refreshed.ID, refreshed.Revision, []string{prerequisite.StableKey})
			return e
		},
	})
	relations, err = r.ListPrerequisites(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetLesson(ctx, prerequisite.ID); errors.Is(err, authoring.ErrNotFound) && len(relations) != 0 {
		t.Fatal("deleted prerequisite resurrected")
	} else if err != nil && !errors.Is(err, authoring.ErrNotFound) {
		t.Fatal(err)
	}

	// Concurrent demotion/revocation cannot remove both of exactly two maintainers.
	for _, kind := range []string{"demote-demote", "demote-revoke", "self-revoke-revoke"} {
		t.Run(kind, func(t *testing.T) {
			d := makeDraft()
			_, d, e := r.AddMemberForDraft(ctx, d.ID, d.Revision, collaborator, authoring.MemberMaintainer)
			if e != nil {
				t.Fatal(e)
			}
			first := func() error {
				_, _, e := r.ChangeMemberRoleForDraft(ctx, d.ID, d.Revision, creator, authoring.MemberAuthor)
				return e
			}
			second := func() error {
				_, _, e := r.ChangeMemberRoleForDraft(ctx, d.ID, d.Revision, collaborator, authoring.MemberAuthor)
				return e
			}
			if kind != "demote-demote" {
				second = func() error { _, _, e := r.RevokeMemberForDraft(ctx, d.ID, d.Revision, collaborator); return e }
			}
			if kind == "self-revoke-revoke" {
				first = func() error { _, _, e := r.RevokeMemberForDraft(ctx, d.ID, d.Revision, creator); return e }
			}
			runAuthoringRace(t, []func() error{first, second})
			workspace, e := r.GetWorkspace(ctx, d.ID)
			if e != nil {
				t.Fatal(e)
			}
			members, e := r.ListMembers(ctx, workspace.ID)
			if e != nil {
				t.Fatal(e)
			}
			remaining := ""
			count := 0
			for _, m := range members {
				if m.Role == authoring.MemberMaintainer && m.RevokedAt == nil {
					remaining = m.UserID
					count++
				}
			}
			if count != 1 {
				t.Fatalf("membership race left %d maintainers", count)
			}
			committed, e := r.GetDraft(ctx, d.ID)
			if e != nil || committed.Revision != d.Revision+1 {
				t.Fatalf("membership race revision = %d: %v", committed.Revision, e)
			}
			if _, _, e := r.RevokeMemberForDraft(ctx, d.ID, committed.Revision, remaining); !errors.Is(e, authoring.ErrConflict) {
				t.Fatalf("final maintainer revoked: %v", e)
			}
			intact, e := r.GetDraft(ctx, d.ID)
			if e != nil || intact.Revision != committed.Revision {
				t.Fatalf("failed member mutation changed revision: %v", e)
			}
		})
	}
	t.Run("duplicate member adds", func(t *testing.T) {
		d := makeDraft()
		runAuthoringRace(t, []func() error{func() error {
			_, _, e := r.AddMemberForDraft(ctx, d.ID, d.Revision, collaborator, authoring.MemberAuthor)
			return e
		}, func() error {
			_, _, e := r.AddMemberForDraft(ctx, d.ID, d.Revision, collaborator, authoring.MemberMaintainer)
			return e
		}})
		workspace, e := r.GetWorkspace(ctx, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		members, e := r.ListMembers(ctx, workspace.ID)
		if e != nil || len(members) != 2 {
			t.Fatalf("concurrent add violated uniqueness: %v %#v", e, members)
		}
	})
}

func runAuthoringRace(t *testing.T, operations []func() error) {
	t.Helper()
	start := make(chan struct{})
	results := make(chan error, len(operations))
	var wait sync.WaitGroup
	for _, operation := range operations {
		wait.Add(1)
		go func(run func() error) { defer wait.Done(); <-start; results <- run() }(operation)
	}
	close(start)
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, authoring.ErrRevisionMismatch) || errors.Is(err, authoring.ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected race failure: %v", err)
		}
	}
	if successes != 1 || conflicts != len(operations)-1 {
		t.Fatalf("race successes=%d conflicts=%d", successes, conflicts)
	}
}

type authoringSnapshotTraceKey struct{}
type authoringSnapshotTracer struct {
	entered, release chan struct{}
	once             sync.Once
}

func (tracer *authoringSnapshotTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, authoringSnapshotTraceKey{}, strings.Contains(data.SQL, "-- name: ListModules :many"))
}
func (tracer *authoringSnapshotTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if selected, _ := ctx.Value(authoringSnapshotTraceKey{}).(bool); !selected {
		return
	}
	tracer.once.Do(func() {
		close(tracer.entered)
		select {
		case <-tracer.release:
		case <-ctx.Done():
		}
	})
}
func testAuthoringReadSnapshot(t *testing.T, parent context.Context, pool *pgxpool.Pool, r *authoringpostgres.Repository, draftID authoring.DraftID, moduleID authoring.ModuleID, lessonID authoring.LessonID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tracer := &authoringSnapshotTracer{entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(tracer.release) })
	config := pool.Config()
	config.ConnConfig.Tracer = tracer
	readerPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		release.Do(func() { close(tracer.release) })
		cancel()
		readerPool.Close()
	}()
	results := make(chan []authoring.ModuleStructure, 1)
	failures := make(chan error, 1)
	go func() {
		result, err := authoringpostgres.New(readerPool).ReadStructure(ctx, draftID)
		results <- result
		failures <- err
	}()
	select {
	case <-tracer.entered:
	case <-ctx.Done():
		t.Fatal("snapshot reader did not reach barrier")
	}
	draft, err := r.GetDraft(ctx, draftID)
	if err != nil {
		t.Fatal(err)
	}
	temporary, draft, err := r.CreateModuleAtPosition(ctx, draft.ID, draft.Revision, authoring.ModuleInput{DraftID: draft.ID, StableKey: "snapshot-temporary", Title: "Temporary", Position: 1})
	if err != nil {
		t.Fatal(err)
	}
	draft, err = r.ReorderLessonsForDraft(ctx, draft.ID, draft.Revision, []authoring.ModuleLessonOrder{{ModuleID: moduleID, LessonIDs: []authoring.LessonID{}}, {ModuleID: temporary.ID, LessonIDs: []authoring.LessonID{lessonID}}})
	if err != nil {
		t.Fatal(err)
	}
	release.Do(func() { close(tracer.release) })
	result := <-results
	if err := <-failures; err != nil || len(result) != 1 || result[0].Module.ID != moduleID || len(result[0].Lessons) != 1 || result[0].Lessons[0].Lesson.ID != lessonID {
		t.Fatalf("outline mixed pre-move Modules with post-move Lessons: %v %#v", err, result)
	}
	draft, err = r.ReorderLessonsForDraft(ctx, draft.ID, draft.Revision, []authoring.ModuleLessonOrder{{ModuleID: moduleID, LessonIDs: []authoring.LessonID{lessonID}}, {ModuleID: temporary.ID, LessonIDs: []authoring.LessonID{}}})
	if err != nil {
		t.Fatal(err)
	}
}
