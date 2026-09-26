package postgres

import (
	"context"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ progress.Repository = (*Repository)(nil)

func (r *Repository) Create(ctx context.Context, learner, version string) (progress.CourseProgress, error) {
	_, e := r.pool.Exec(ctx, `INSERT INTO progress.course_progress (learner_user_id,course_version_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, learner, version)
	if e != nil {
		return progress.CourseProgress{}, storage(e)
	}
	return r.Get(ctx, learner, version)
}
func (r *Repository) Get(ctx context.Context, learner, version string) (progress.CourseProgress, error) {
	var p progress.CourseProgress
	p.LearnerUserID = learner
	p.CourseVersionID = version
	rows, e := r.pool.Query(ctx, `SELECT revision,created_at,updated_at FROM progress.course_progress WHERE learner_user_id=$1 AND course_version_id=$2`, learner, version)
	if e != nil {
		return p, storage(e)
	}
	defer rows.Close()
	if !rows.Next() {
		return p, progress.ErrProgressNotFound
	}
	if e = rows.Scan(&p.Revision, &p.CreatedAt, &p.UpdatedAt); e != nil {
		return p, storage(e)
	}
	rows.Close()
	rs, e := r.pool.Query(ctx, `SELECT lesson_key FROM progress.lesson_completion WHERE learner_user_id=$1 AND course_version_id=$2 ORDER BY lesson_key`, learner, version)
	if e != nil {
		return p, storage(e)
	}
	defer rs.Close()
	for rs.Next() {
		var k string
		if e = rs.Scan(&k); e != nil {
			return p, storage(e)
		}
		p.CompletedLessonKeys = append(p.CompletedLessonKeys, k)
	}
	if e = rs.Err(); e != nil {
		return p, storage(e)
	}
	if p.CompletedLessonKeys == nil {
		p.CompletedLessonKeys = []string{}
	}
	return p, p.Validate()
}
func (r *Repository) MarkLessonCompleted(ctx context.Context, learner, version, key string, expected int64) (progress.CourseProgress, error) {
	if expected < 1 {
		return progress.CourseProgress{}, progress.ErrInvalidProgress
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return progress.CourseProgress{}, storage(e)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var revision int64
	e = tx.QueryRow(ctx, `SELECT revision FROM progress.course_progress WHERE learner_user_id=$1 AND course_version_id=$2 FOR UPDATE`, learner, version).Scan(&revision)
	if errors.Is(e, pgx.ErrNoRows) {
		return progress.CourseProgress{}, progress.ErrProgressNotFound
	}
	if e != nil {
		return progress.CourseProgress{}, storage(e)
	}
	if revision != expected {
		return progress.CourseProgress{}, progress.ErrRevisionMismatch
	}
	tag, e := tx.Exec(ctx, `INSERT INTO progress.lesson_completion (learner_user_id,course_version_id,lesson_key,completed_at) VALUES ($1,$2,$3,now()) ON CONFLICT DO NOTHING`, learner, version, key)
	if e != nil {
		return progress.CourseProgress{}, storage(e)
	}
	if tag.RowsAffected() > 0 {
		_, e = tx.Exec(ctx, `UPDATE progress.course_progress SET revision=revision+1,updated_at=now() WHERE learner_user_id=$1 AND course_version_id=$2`, learner, version)
		if e != nil {
			return progress.CourseProgress{}, storage(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return progress.CourseProgress{}, storage(e)
	}
	return r.Get(ctx, learner, version)
}
func storage(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return progress.ErrProgressNotFound
	}
	var pe *pgconn.PgError
	if errors.As(e, &pe) {
		if pe.Code == "23503" || pe.Code == "23514" || pe.Code == "22P02" {
			return progress.ErrInvalidProgress
		}
	}
	return e
}
