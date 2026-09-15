package postgres

import (
	"context"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/db/sqlc"
	"github.com/jackc/pgx/v5"
)

// ReadStructure uses one MVCC snapshot and a metadata-only Lesson projection.
// A concurrent move/delete cannot combine an old Module list with new Lessons.
func (r *Repository) ReadStructure(ctx context.Context, draftID authoring.DraftID) ([]authoring.ModuleStructure, error) {
	id, err := uuid(string(draftID))
	if err != nil {
		return nil, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, storageError(err)
	}
	defer tx.Rollback(ctx)
	q := r.q.WithTx(tx)
	if _, err := q.GetDraft(ctx, id); err != nil {
		return nil, storageError(err)
	}
	modules, err := q.ListModules(ctx, id)
	if err != nil {
		return nil, storageError(err)
	}
	lessons, err := q.ListLessonSummariesForDraft(ctx, id)
	if err != nil {
		return nil, storageError(err)
	}
	prerequisites, err := q.ListPrerequisitesForDraft(ctx, id)
	if err != nil {
		return nil, storageError(err)
	}
	keys := make(map[authoring.LessonID][]string)
	for _, p := range prerequisites {
		source := authoring.LessonID(p.LessonID.String())
		keys[source] = append(keys[source], p.TargetStableKey)
	}
	byModule := make(map[authoring.ModuleID][]authoring.LessonStructure)
	for _, row := range lessons {
		lesson, err := mapLesson(sqlc.AuthoringLesson(row))
		if err != nil {
			return nil, err
		}
		prerequisiteKeys := keys[lesson.ID]
		if prerequisiteKeys == nil {
			prerequisiteKeys = []string{}
		}
		byModule[lesson.ModuleID] = append(byModule[lesson.ModuleID], authoring.LessonStructure{Lesson: lesson, RecommendedPrerequisiteKeys: prerequisiteKeys})
	}
	result := make([]authoring.ModuleStructure, 0, len(modules))
	for _, row := range modules {
		module := mapModule(row)
		result = append(result, authoring.ModuleStructure{Module: module, Lessons: byModule[module.ID]})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, storageError(err)
	}
	return result, nil
}

// ReadLesson scopes the content query before decoding and reads its advisory
// prerequisites in the same snapshot as the Lesson revision.
func (r *Repository) ReadLesson(ctx context.Context, draftID authoring.DraftID, lessonID authoring.LessonID) (authoring.DraftLesson, []authoring.Prerequisite, error) {
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.DraftLesson{}, nil, err
	}
	lessonKey, err := uuid(string(lessonID))
	if err != nil {
		return authoring.DraftLesson{}, nil, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return authoring.DraftLesson{}, nil, storageError(err)
	}
	defer tx.Rollback(ctx)
	q := r.q.WithTx(tx)
	row, err := q.GetLessonForDraft(ctx, sqlc.GetLessonForDraftParams{DraftID: draftKey, ID: lessonKey})
	if err != nil {
		return authoring.DraftLesson{}, nil, storageError(err)
	}
	lesson, err := mapLesson(row)
	if err != nil {
		return authoring.DraftLesson{}, nil, err
	}
	rows, err := q.ListPrerequisites(ctx, lessonKey)
	if err != nil {
		return authoring.DraftLesson{}, nil, storageError(err)
	}
	prerequisites := make([]authoring.Prerequisite, 0, len(rows))
	for _, p := range rows {
		prerequisites = append(prerequisites, authoring.Prerequisite{LessonID: lessonID, TargetLessonID: authoring.LessonID(p.PrerequisiteLessonID.String()), TargetStableKey: p.TargetStableKey, Position: int(p.Position)})
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftLesson{}, nil, storageError(err)
	}
	return lesson, prerequisites, nil
}
