package postgres

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/db/sqlc"
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
	row, err := q.BumpDraftRevision(ctx, sqlc.BumpDraftRevisionParams{ID: id, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.CourseDraft{}, r.draftMiss(ctx, id)
	}
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	modules, err := q.ListModules(ctx, id)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	got := make([]authoring.ModuleID, 0, len(modules))
	for _, m := range modules {
		got = append(got, authoring.ModuleID(m.ID.String()))
	}
	if !sameIDs(got, order) {
		return authoring.CourseDraft{}, authoring.ErrConflict
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
