package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/db/sqlc"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repository) lockActiveDraft(ctx context.Context, tx pgx.Tx, id pgtype.UUID) error {
	var status string
	if err := tx.QueryRow(ctx, "SELECT status FROM authoring.course_draft WHERE id = $1 FOR UPDATE", id).Scan(&status); err != nil {
		return storageError(err)
	}
	if status != string(authoring.DraftActive) {
		return authoring.ErrInvalidState
	}
	return nil
}

func (r *Repository) lockActiveDraftRevision(ctx context.Context, tx pgx.Tx, id pgtype.UUID, expected int64) error {
	var status string
	var revision int64
	if err := tx.QueryRow(ctx, "SELECT status, revision FROM authoring.course_draft WHERE id = $1 FOR UPDATE", id).Scan(&status, &revision); err != nil {
		return storageError(err)
	}
	if status != string(authoring.DraftActive) {
		return authoring.ErrInvalidState
	}
	if revision != expected {
		return authoring.ErrRevisionMismatch
	}
	return nil
}

func sameIDs[T ~string](got []T, wanted []T) bool {
	if len(got) != len(wanted) {
		return false
	}
	seen := make(map[T]bool, len(got))
	for _, id := range got {
		seen[id] = true
	}
	if len(seen) != len(got) {
		return false
	}
	for _, id := range wanted {
		if !seen[id] {
			return false
		}
	}
	return true
}

func (r *Repository) ReorderModules(ctx context.Context, draftID authoring.DraftID, expected int64, order []authoring.ModuleID) (authoring.CourseDraft, error) {
	if err := authoring.ValidateModuleOrder(order); err != nil {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	id, err := uuid(string(draftID))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	q := r.q.WithTx(tx)
	if err := r.lockActiveDraft(ctx, tx, id); err != nil {
		return authoring.CourseDraft{}, err
	}
	modules, err := q.ListModules(ctx, id)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	got := make([]authoring.ModuleID, 0, len(modules))
	for _, m := range modules {
		got = append(got, authoring.ModuleID(m.ID.String()))
	}
	for _, requested := range order {
		found := false
		for _, actual := range got {
			if actual == requested {
				found = true
				break
			}
		}
		if !found {
			return authoring.CourseDraft{}, authoring.ErrNotFound
		}
	}
	if !sameIDs(got, order) {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	if err := r.lockActiveDraftRevision(ctx, tx, id, expected); err != nil {
		return authoring.CourseDraft{}, err
	}
	row, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: id, Revision: expected})
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_module_draft_position_unique DEFERRED"); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	for pos, moduleID := range order {
		key, err := uuid(string(moduleID))
		if err != nil {
			return authoring.CourseDraft{}, err
		}
		if _, err := q.SetModulePosition(ctx, sqlc.SetModulePositionParams{ID: key, Position: int32(pos)}); err != nil {
			return authoring.CourseDraft{}, storageError(err)
		}
	}
	if err := q.TouchWorkspace(ctx, id); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	return mapDraft(row)
}

// CreateModuleAtPosition inserts a module at a contiguous zero-based position.
// The parent draft row is locked and compared before any existing positions are
// shifted, so a stale structural client cannot partially alter the outline.
func (r *Repository) CreateModuleAtPosition(ctx context.Context, draftID authoring.DraftID, expected int64, input authoring.ModuleInput) (authoring.DraftModule, authoring.CourseDraft, error) {
	if input.DraftID != draftID || input.Validate() != nil || expected < 1 {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	id, err := uuid(string(draftID))
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraftRevision(ctx, tx, id, expected); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	count, err := q.CountModules(ctx, id)
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	if count >= authoring.MaxModulesPerDraft || input.Position > int(count) {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_module_draft_position_unique DEFERRED"); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.ShiftModulesAtPosition(ctx, sqlc.ShiftModulesAtPositionParams{DraftID: id, Position: int32(input.Position)}); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	moduleRow, err := q.CreateModule(ctx, sqlc.CreateModuleParams{ID: id, StableKey: input.StableKey, Title: input.Title, Description: input.Description, Position: int32(input.Position)})
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: id, Revision: expected})
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, id); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, err
	}
	return mapModule(moduleRow), draft, nil
}

// UpdateModuleMetadata applies a metadata-only compare-and-swap. Stable keys
// and positions are deliberately absent from the patch.
func (r *Repository) UpdateModuleMetadata(ctx context.Context, draftID authoring.DraftID, moduleID authoring.ModuleID, expected int64, patch authoring.DraftModulePatch) (authoring.DraftModule, authoring.CourseDraft, error) {
	if expected < 1 || patch.Empty() {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, err
	}
	moduleKey, err := uuid(string(moduleID))
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	currentRow, err := q.GetModule(ctx, moduleKey)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && currentRow.DraftID != draftKey) {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	current := mapModule(currentRow)
	next, err := patch.Apply(current.ModuleInput)
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	moduleRow, err := q.UpdateModule(ctx, sqlc.UpdateModuleParams{ID: moduleKey, Revision: expected, Title: next.Title, Description: next.Description})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftModule{}, authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	// The draft is locked, so reading its current revision and CAS-bumping it
	// gives the response the exact committed aggregate revision.
	var draftRevision int64
	if err := tx.QueryRow(ctx, "SELECT revision FROM authoring.course_draft WHERE id = $1", draftKey).Scan(&draftRevision); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: draftRevision})
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, storageError(err)
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.DraftModule{}, authoring.CourseDraft{}, err
	}
	return mapModule(moduleRow), draft, nil
}

// DeleteEmptyModule deliberately refuses to expose the persistence cascade for
// contained lessons. Lesson movement/deletion remains an explicit later API.
func (r *Repository) DeleteEmptyModule(ctx context.Context, draftID authoring.DraftID, moduleID authoring.ModuleID, expectedDraft, expectedModule int64) (authoring.CourseDraft, error) {
	if expectedDraft < 1 || expectedModule < 1 {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	moduleKey, err := uuid(string(moduleID))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	current, err := q.GetModule(ctx, moduleKey)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.DraftID != draftKey) {
		return authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := r.lockActiveDraftRevision(ctx, tx, draftKey, expectedDraft); err != nil {
		return authoring.CourseDraft{}, err
	}
	lessons, err := q.CountLessonsForModule(ctx, moduleKey)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if lessons > 0 {
		return authoring.CourseDraft{}, authoring.ErrConflict
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_module_draft_position_unique DEFERRED"); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if _, err := q.DeleteModuleForDraft(ctx, sqlc.DeleteModuleForDraftParams{ID: moduleKey, DraftID: draftKey, Revision: expectedModule}); errors.Is(err, pgx.ErrNoRows) {
		return authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	} else if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.CompactModulePositionsAfter(ctx, sqlc.CompactModulePositionsAfterParams{DraftID: draftKey, Position: current.Position}); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: expectedDraft})
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	return mapDraft(draftRow)
}

func (r *Repository) CreateLessonAtPosition(ctx context.Context, draftID authoring.DraftID, moduleID authoring.ModuleID, expected int64, input authoring.LessonInput) (authoring.DraftLesson, authoring.CourseDraft, error) {
	if input.DraftID != draftID || input.ModuleID != moduleID || input.Validate() != nil || expected < 1 {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	moduleKey, err := uuid(string(moduleID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	objectives, err := json.Marshal(input.LearningObjectives)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	content, err := courses.MarshalLessonContent(input.Content)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	module, err := q.GetModule(ctx, moduleKey)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && module.DraftID != draftKey) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := r.lockActiveDraftRevision(ctx, tx, draftKey, expected); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	count, err := q.CountLessonsForModule(ctx, moduleKey)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	total, err := q.CountLessonsForDraft(ctx, draftKey)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if total >= authoring.MaxLessonsPerDraft || input.Position > int(count) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_lesson_module_position_unique DEFERRED"); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.ShiftLessonsAtPosition(ctx, sqlc.ShiftLessonsAtPositionParams{ModuleID: moduleKey, Position: int32(input.Position)}); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	lessonRow, err := q.CreateLesson(ctx, sqlc.CreateLessonParams{ID: moduleKey, DraftID: draftKey, StableKey: input.StableKey, Title: input.Title, Description: input.Description, LearningObjectives: objectives, EstimatedDurationMinutes: duration(input.EstimatedDurationMinutes), Position: int32(input.Position), Content: content})
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if _, err := q.BumpModuleRevision(ctx, sqlc.BumpModuleRevisionParams{ID: moduleKey, Revision: module.Revision}); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: expected})
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	lesson, err := mapLesson(lessonRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	return lesson, draft, nil
}

func (r *Repository) UpdateLessonMetadataForDraft(ctx context.Context, draftID authoring.DraftID, lessonID authoring.LessonID, expected int64, patch authoring.DraftLessonPatch) (authoring.DraftLesson, authoring.CourseDraft, error) {
	if expected < 1 || patch.Empty() {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	lessonKey, err := uuid(string(lessonID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	currentRow, err := q.GetLesson(ctx, lessonKey)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && currentRow.DraftID != draftKey) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	current, err := mapLesson(currentRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	next, err := patch.Apply(current.LessonInput)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	objectives, err := json.Marshal(next.LearningObjectives)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	lessonRow, err := q.UpdateLessonMetadata(ctx, sqlc.UpdateLessonMetadataParams{ID: lessonKey, Revision: expected, Title: next.Title, Description: next.Description, LearningObjectives: objectives, EstimatedDurationMinutes: duration(next.EstimatedDurationMinutes)})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	var revision int64
	if err := tx.QueryRow(ctx, "SELECT revision FROM authoring.course_draft WHERE id = $1", draftKey).Scan(&revision); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: revision})
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	lesson, err := mapLesson(lessonRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	return lesson, draft, nil
}

// ReplaceLessonContentForDraft atomically replaces the complete canonical
// semantic document for one draft-scoped Lesson. The Lesson revision is the
// compare-and-swap guard; the locked Draft receives one aggregate revision.
func (r *Repository) ReplaceLessonContentForDraft(ctx context.Context, draftID authoring.DraftID, lessonID authoring.LessonID, expected int64, content courses.LessonContent) (authoring.DraftLesson, authoring.CourseDraft, error) {
	if expected < 1 {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	encoded, err := courses.MarshalLessonContent(content)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	lessonKey, err := uuid(string(lessonID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	current, err := q.GetLesson(ctx, lessonKey)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.DraftID != draftKey) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	lessonRow, err := q.UpdateLessonContent(ctx, sqlc.UpdateLessonContentParams{ID: lessonKey, Revision: expected, Content: encoded})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	var revision int64
	if err := tx.QueryRow(ctx, "SELECT revision FROM authoring.course_draft WHERE id = $1", draftKey).Scan(&revision); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: revision})
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	lesson, err := mapLesson(lessonRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	return lesson, draft, nil
}

func (r *Repository) ReorderLessonsForDraft(ctx context.Context, draftID authoring.DraftID, expected int64, order []authoring.ModuleLessonOrder) (authoring.CourseDraft, error) {
	if expected < 1 || authoring.ValidateLessonOrder(order) != nil {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	modules, err := q.ListModules(ctx, draftKey)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	moduleIDs := make([]authoring.ModuleID, 0, len(modules))
	moduleRows := make(map[authoring.ModuleID]sqlc.AuthoringModule, len(modules))
	for _, module := range modules {
		id := authoring.ModuleID(module.ID.String())
		moduleIDs = append(moduleIDs, id)
		moduleRows[id] = module
	}
	requestedModules := make([]authoring.ModuleID, 0, len(order))
	for _, item := range order {
		if _, found := moduleRows[item.ModuleID]; !found {
			return authoring.CourseDraft{}, authoring.ErrNotFound
		}
		requestedModules = append(requestedModules, item.ModuleID)
	}
	if !sameIDs(moduleIDs, requestedModules) {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	lessons, err := q.ListLessonSummariesForDraft(ctx, draftKey)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	lessonRows := make(map[authoring.LessonID]sqlc.AuthoringLesson, len(lessons))
	requestedLessons := make([]authoring.LessonID, 0, len(lessons))
	for _, item := range order {
		requestedLessons = append(requestedLessons, item.LessonIDs...)
	}
	for _, lesson := range lessons {
		lessonRows[authoring.LessonID(lesson.ID.String())] = sqlc.AuthoringLesson(lesson)
	}
	actualLessons := make([]authoring.LessonID, 0, len(lessons))
	for _, lesson := range lessons {
		actualLessons = append(actualLessons, authoring.LessonID(lesson.ID.String()))
	}
	for _, lessonID := range requestedLessons {
		if _, found := lessonRows[lessonID]; !found {
			return authoring.CourseDraft{}, authoring.ErrNotFound
		}
	}
	if !sameIDs(actualLessons, requestedLessons) {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	if err := r.lockActiveDraftRevision(ctx, tx, draftKey, expected); err != nil {
		return authoring.CourseDraft{}, err
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_lesson_module_position_unique DEFERRED"); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	changedModules := make(map[authoring.ModuleID]struct{})
	for _, item := range order {
		module := moduleRows[item.ModuleID]
		for position, lessonID := range item.LessonIDs {
			lesson := lessonRows[lessonID]
			if lesson.ModuleID == module.ID && lesson.Position == int32(position) {
				continue
			}
			if _, err := q.SetLessonModuleAndPosition(ctx, sqlc.SetLessonModuleAndPositionParams{ID: lesson.ID, ModuleID: module.ID, Position: int32(position), Revision: lesson.Revision}); err != nil {
				return authoring.CourseDraft{}, storageError(err)
			}
			changedModules[item.ModuleID] = struct{}{}
			changedModules[authoring.ModuleID(lesson.ModuleID.String())] = struct{}{}
		}
	}
	for moduleID := range changedModules {
		module := moduleRows[moduleID]
		if _, err := q.BumpModuleRevision(ctx, sqlc.BumpModuleRevisionParams{ID: module.ID, Revision: module.Revision}); err != nil {
			return authoring.CourseDraft{}, storageError(err)
		}
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: expected})
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	return mapDraft(draftRow)
}

func (r *Repository) ReplaceLessonPrerequisitesForDraft(ctx context.Context, draftID authoring.DraftID, lessonID authoring.LessonID, expected int64, keys []string) (authoring.DraftLesson, authoring.CourseDraft, error) {
	if expected < 1 || authoring.ValidatePrerequisiteKeys("", keys) != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	lessonKey, err := uuid(string(lessonID))
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	currentRow, err := q.GetLesson(ctx, lessonKey)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && currentRow.DraftID != draftKey) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := authoring.ValidatePrerequisiteKeys(currentRow.StableKey, keys); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	targets := make([]sqlc.AuthoringLesson, 0, len(keys))
	for _, key := range keys {
		target, err := q.FindLessonByDraftAndKey(ctx, sqlc.FindLessonByDraftAndKeyParams{DraftID: draftKey, StableKey: key})
		if errors.Is(err, pgx.ErrNoRows) {
			return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrConflict
		}
		if err != nil {
			return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
		}
		targets = append(targets, target)
	}
	lessonRow, err := q.BumpLessonRevision(ctx, sqlc.BumpLessonRevisionParams{ID: lessonKey, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	}
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.DeletePrerequisites(ctx, lessonKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	for position, target := range targets {
		if err := q.AddPrerequisite(ctx, sqlc.AddPrerequisiteParams{DraftID: draftKey, LessonID: lessonKey, PrerequisiteLessonID: target.ID, Position: int32(position)}); err != nil {
			return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
		}
	}
	var revision int64
	if err := tx.QueryRow(ctx, "SELECT revision FROM authoring.course_draft WHERE id = $1", draftKey).Scan(&revision); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: revision})
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, storageError(err)
	}
	lesson, err := mapLesson(lessonRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.DraftLesson{}, authoring.CourseDraft{}, err
	}
	return lesson, draft, nil
}

func (r *Repository) DeleteLessonForDraft(ctx context.Context, draftID authoring.DraftID, lessonID authoring.LessonID, expectedDraft, expectedLesson int64) (authoring.CourseDraft, error) {
	if expectedDraft < 1 || expectedLesson < 1 {
		return authoring.CourseDraft{}, authoring.ErrInvalidStructure
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	lessonKey, err := uuid(string(lessonID))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftKey); err != nil {
		return authoring.CourseDraft{}, err
	}
	q := r.q.WithTx(tx)
	current, err := q.GetLesson(ctx, lessonKey)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.DraftID != draftKey) {
		return authoring.CourseDraft{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := r.lockActiveDraftRevision(ctx, tx, draftKey, expectedDraft); err != nil {
		return authoring.CourseDraft{}, err
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_lesson_module_position_unique DEFERRED"); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.AdvanceLessonsAfterDeletion(ctx, sqlc.AdvanceLessonsAfterDeletionParams{ID: lessonKey, ModuleID: current.ModuleID, Position: current.Position}); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.DeleteIncomingPrerequisites(ctx, lessonKey); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.DeletePrerequisites(ctx, lessonKey); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if _, err := q.DeleteLessonForDraft(ctx, sqlc.DeleteLessonForDraftParams{ID: lessonKey, DraftID: draftKey, Revision: expectedLesson}); errors.Is(err, pgx.ErrNoRows) {
		return authoring.CourseDraft{}, authoring.ErrRevisionMismatch
	} else if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	module, err := q.GetModule(ctx, current.ModuleID)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if _, err := q.BumpModuleRevision(ctx, sqlc.BumpModuleRevisionParams{ID: module.ID, Revision: module.Revision}); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	draftRow, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: draftKey, Revision: expectedDraft})
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftKey); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	return mapDraft(draftRow)
}

func (r *Repository) ReorderLessons(ctx context.Context, moduleID authoring.ModuleID, expected int64, order []authoring.LessonID) (authoring.DraftModule, error) {
	module, err := r.GetModule(ctx, moduleID)
	if err != nil {
		return authoring.DraftModule{}, err
	}
	id, err := uuid(string(moduleID))
	if err != nil {
		return authoring.DraftModule{}, err
	}
	draftID, err := uuid(string(module.DraftID))
	if err != nil {
		return authoring.DraftModule{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return authoring.DraftModule{}, err
	}
	q := r.q.WithTx(tx)
	row, err := q.BumpModuleRevision(ctx, sqlc.BumpModuleRevisionParams{ID: id, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftModule{}, r.moduleMiss(ctx, id)
	}
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	lessons, err := q.ListLessons(ctx, id)
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	got := make([]authoring.LessonID, 0, len(lessons))
	for _, l := range lessons {
		got = append(got, authoring.LessonID(l.ID.String()))
	}
	if !sameIDs(got, order) {
		return authoring.DraftModule{}, authoring.ErrConflict
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_lesson_module_position_unique DEFERRED"); err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	for pos, lessonID := range order {
		key, err := uuid(string(lessonID))
		if err != nil {
			return authoring.DraftModule{}, err
		}
		if _, err := q.SetLessonPosition(ctx, sqlc.SetLessonPositionParams{ID: key, Position: int32(pos)}); err != nil {
			return authoring.DraftModule{}, storageError(err)
		}
	}
	if err := q.TouchDraft(ctx, draftID); err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftID); err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	return mapModule(row), nil
}

func (r *Repository) DeleteModule(ctx context.Context, moduleID authoring.ModuleID, expected int64) error {
	id, err := uuid(string(moduleID))
	if err != nil {
		return err
	}
	current, err := r.GetModule(ctx, moduleID)
	if err != nil {
		return err
	}
	draftID, err := uuid(string(current.DraftID))
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return err
	}
	q := r.q.WithTx(tx)
	_, err = q.DeleteModule(ctx, sqlc.DeleteModuleParams{ID: id, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.moduleMiss(ctx, id)
	}
	if err != nil {
		return storageError(err)
	}
	if err := q.TouchDraft(ctx, draftID); err != nil {
		return storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftID); err != nil {
		return storageError(err)
	}
	return storageError(tx.Commit(ctx))
}

func (r *Repository) DeleteLesson(ctx context.Context, lessonID authoring.LessonID, expected int64) error {
	id, err := uuid(string(lessonID))
	if err != nil {
		return err
	}
	current, err := r.GetLesson(ctx, lessonID)
	if err != nil {
		return err
	}
	draftID, err := uuid(string(current.DraftID))
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return err
	}
	q := r.q.WithTx(tx)
	_, err = q.DeleteLesson(ctx, sqlc.DeleteLessonParams{ID: id, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.lessonMiss(ctx, id)
	}
	if err != nil {
		return storageError(err)
	}
	if err := q.TouchDraft(ctx, draftID); err != nil {
		return storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftID); err != nil {
		return storageError(err)
	}
	return storageError(tx.Commit(ctx))
}

func (r *Repository) ReplacePrerequisites(ctx context.Context, lessonID authoring.LessonID, expected int64, keys []string) (authoring.DraftLesson, error) {
	current, err := r.GetLesson(ctx, lessonID)
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	if err := authoring.ValidatePrerequisiteKeys(current.StableKey, keys); err != nil {
		return authoring.DraftLesson{}, err
	}
	id, err := uuid(string(lessonID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	draftID, err := uuid(string(current.DraftID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return authoring.DraftLesson{}, err
	}
	q := r.q.WithTx(tx)
	row, err := q.BumpLessonRevision(ctx, sqlc.BumpLessonRevisionParams{ID: id, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftLesson{}, r.lessonMiss(ctx, id)
	}
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	if err := q.DeletePrerequisites(ctx, id); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	for pos, key := range keys {
		target, err := q.FindLessonByDraftAndKey(ctx, sqlc.FindLessonByDraftAndKeyParams{DraftID: draftID, StableKey: key})
		if errors.Is(err, pgx.ErrNoRows) {
			return authoring.DraftLesson{}, authoring.ErrNotFound
		}
		if err != nil {
			return authoring.DraftLesson{}, storageError(err)
		}
		if err := q.AddPrerequisite(ctx, sqlc.AddPrerequisiteParams{DraftID: draftID, LessonID: id, PrerequisiteLessonID: target.ID, Position: int32(pos)}); err != nil {
			return authoring.DraftLesson{}, storageError(err)
		}
	}
	if err := q.TouchDraft(ctx, draftID); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftID); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	return mapLesson(row)
}

func (r *Repository) ListPrerequisites(ctx context.Context, lessonID authoring.LessonID) ([]authoring.Prerequisite, error) {
	id, err := uuid(string(lessonID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListPrerequisites(ctx, id)
	if err != nil {
		return nil, storageError(err)
	}
	out := make([]authoring.Prerequisite, 0, len(rows))
	for _, row := range rows {
		out = append(out, authoring.Prerequisite{LessonID: authoring.LessonID(row.LessonID.String()), TargetLessonID: authoring.LessonID(row.PrerequisiteLessonID.String()), TargetStableKey: row.TargetStableKey, Position: int(row.Position)})
	}
	return out, nil
}

func (r *Repository) ListPrerequisitesForDraft(ctx context.Context, draftID authoring.DraftID) ([]authoring.Prerequisite, error) {
	id, err := uuid(string(draftID))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListPrerequisitesForDraft(ctx, id)
	if err != nil {
		return nil, storageError(err)
	}
	out := make([]authoring.Prerequisite, 0, len(rows))
	for _, row := range rows {
		out = append(out, authoring.Prerequisite{LessonID: authoring.LessonID(row.LessonID.String()), TargetLessonID: authoring.LessonID(row.PrerequisiteLessonID.String()), TargetStableKey: row.TargetStableKey, Position: int(row.Position)})
	}
	return out, nil
}

// MoveLesson renumbers only the two affected modules. The draft row lock
// serializes structural mutations, while deferrable uniqueness permits swaps.
func (r *Repository) MoveLesson(ctx context.Context, lessonID authoring.LessonID, expected int64, targetModuleID authoring.ModuleID, targetPosition int) (authoring.DraftLesson, error) {
	current, err := r.GetLesson(ctx, lessonID)
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	target, err := r.GetModule(ctx, targetModuleID)
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	if current.DraftID != target.DraftID {
		return authoring.DraftLesson{}, authoring.ErrConflict
	}
	if targetPosition < 0 || targetPosition > 100000 {
		return authoring.DraftLesson{}, authoring.ErrConflict
	}
	draftID, err := uuid(string(current.DraftID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	currentID, err := uuid(string(lessonID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	targetID, err := uuid(string(targetModuleID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	sourceID, err := uuid(string(current.ModuleID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	defer tx.Rollback(ctx)
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return authoring.DraftLesson{}, err
	}
	q := r.q.WithTx(tx)
	source, err := q.ListLessons(ctx, sourceID)
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	var dest []sqlc.AuthoringLesson
	if sourceID == targetID {
		dest = source
	} else {
		dest, err = q.ListLessons(ctx, targetID)
		if err != nil {
			return authoring.DraftLesson{}, storageError(err)
		}
	}
	if _, err := tx.Exec(ctx, "SET CONSTRAINTS authoring.authoring_lesson_module_position_unique DEFERRED"); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	without := func(rows []sqlc.AuthoringLesson) []sqlc.AuthoringLesson {
		out := make([]sqlc.AuthoringLesson, 0, len(rows))
		for _, row := range rows {
			if row.ID != currentID {
				out = append(out, row)
			}
		}
		return out
	}
	source = without(source)
	dest = without(dest)
	if targetPosition > len(dest) {
		return authoring.DraftLesson{}, authoring.ErrConflict
	}
	row, err := q.SetLessonModuleAndPosition(ctx, sqlc.SetLessonModuleAndPositionParams{ID: currentID, ModuleID: targetID, Position: int32(targetPosition), Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftLesson{}, r.lessonMiss(ctx, currentID)
	}
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	if sourceID != targetID {
		for pos, item := range source {
			if item.Position != int32(pos) {
				if _, err := q.SetLessonPosition(ctx, sqlc.SetLessonPositionParams{ID: item.ID, Position: int32(pos)}); err != nil {
					return authoring.DraftLesson{}, storageError(err)
				}
			}
		}
	}
	for pos, item := range dest {
		if pos >= targetPosition {
			pos++
		}
		if item.Position != int32(pos) {
			if _, err := q.SetLessonPosition(ctx, sqlc.SetLessonPositionParams{ID: item.ID, Position: int32(pos)}); err != nil {
				return authoring.DraftLesson{}, storageError(err)
			}
		}
	}
	if err := q.TouchDraft(ctx, draftID); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, draftID); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	return mapLesson(row)
}
