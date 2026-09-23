package postgres

import (
	"context"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/community"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(p *pgxpool.Pool) *Repository { return &Repository{p} }

var _ community.Repository = (*Repository)(nil)

func (r *Repository) CreateCommunity(c context.Context, course string, mode community.Mode) (community.CourseCommunity, error) {
	var x community.CourseCommunity
	e := r.pool.QueryRow(c, `INSERT INTO community.course_community(course_id,mode) VALUES($1,$2) RETURNING course_id,mode,created_at,updated_at`, course, mode).Scan(&x.CourseID, &x.Mode, &x.CreatedAt, &x.UpdatedAt)
	return x, err(e)
}
func (r *Repository) GetCommunity(c context.Context, course string) (community.CourseCommunity, error) {
	var x community.CourseCommunity
	e := r.pool.QueryRow(c, `SELECT course_id,mode,created_at,updated_at FROM community.course_community WHERE course_id=$1`, course).Scan(&x.CourseID, &x.Mode, &x.CreatedAt, &x.UpdatedAt)
	return x, err(e)
}
func (r *Repository) CreateThread(c context.Context, in community.ThreadInput) (community.Thread, community.Post, error) {
	if in.Validate() != nil {
		return community.Thread{}, community.Post{}, community.ErrInvalid
	}
	title, _ := community.NormalizeTitle(in.Title)
	body, _ := community.NormalizeBody(in.OpeningBody)
	tx, e := r.pool.Begin(c)
	if e != nil {
		return community.Thread{}, community.Post{}, e
	}
	defer func() { _ = tx.Rollback(c) }()
	var t community.Thread
	e = tx.QueryRow(c, `INSERT INTO community.thread(course_id,title,created_by_user_id) VALUES($1,$2,$3) RETURNING id,course_id,title,created_by_user_id,state,created_at,updated_at`, in.CourseID, title, in.CreatedByUserID).Scan(&t.ID, &t.CourseID, &t.Title, &t.CreatedByUserID, &t.State, &t.CreatedAt, &t.UpdatedAt)
	if e != nil {
		return t, community.Post{}, err(e)
	}
	var p community.Post
	e = tx.QueryRow(c, `INSERT INTO community.post(thread_id,author_user_id,body) VALUES($1,$2,$3) RETURNING id,thread_id,author_user_id,body,state,created_at,updated_at`, t.ID, in.CreatedByUserID, body).Scan(&p.ID, &p.ThreadID, &p.AuthorUserID, &p.Body, &p.State, &p.CreatedAt, &p.UpdatedAt)
	if e != nil {
		return t, p, err(e)
	}
	if e = tx.Commit(c); e != nil {
		return t, p, e
	}
	return t, p, nil
}
func (r *Repository) GetThread(c context.Context, id string) (community.Thread, error) {
	var t community.Thread
	e := r.pool.QueryRow(c, `SELECT id,course_id,title,created_by_user_id,state,created_at,updated_at FROM community.thread WHERE id=$1`, id).Scan(&t.ID, &t.CourseID, &t.Title, &t.CreatedByUserID, &t.State, &t.CreatedAt, &t.UpdatedAt)
	return t, err(e)
}
func (r *Repository) GetPost(c context.Context, id string) (community.Post, error) {
	var p community.Post
	e := r.pool.QueryRow(c, `SELECT id,thread_id,author_user_id,body,state,created_at,updated_at FROM community.post WHERE id=$1`, id).Scan(&p.ID, &p.ThreadID, &p.AuthorUserID, &p.Body, &p.State, &p.CreatedAt, &p.UpdatedAt)
	return p, err(e)
}
func (r *Repository) ListVisibleThreads(c context.Context, course string, limit, offset int) ([]community.ThreadSummary, int, error) {
	var total int
	if e := r.pool.QueryRow(c, `SELECT count(*) FROM community.thread WHERE course_id=$1 AND state='VISIBLE'`, course).Scan(&total); e != nil {
		return nil, 0, err(e)
	}
	rows, e := r.pool.Query(c, `SELECT t.id,t.course_id,t.title,t.created_by_user_id,t.state,t.created_at,t.updated_at,count(p.id) FROM community.thread t LEFT JOIN community.post p ON p.thread_id=t.id AND p.state='VISIBLE' WHERE t.course_id=$1 AND t.state='VISIBLE' GROUP BY t.id ORDER BY t.updated_at DESC,t.id DESC LIMIT $2 OFFSET $3`, course, limit, offset)
	if e != nil {
		return nil, 0, err(e)
	}
	defer rows.Close()
	result := []community.ThreadSummary{}
	for rows.Next() {
		var x community.ThreadSummary
		if e := rows.Scan(&x.ID, &x.CourseID, &x.Title, &x.CreatedByUserID, &x.State, &x.CreatedAt, &x.UpdatedAt, &x.PostCount); e != nil {
			return nil, 0, err(e)
		}
		result = append(result, x)
	}
	if e := rows.Err(); e != nil {
		return nil, 0, err(e)
	}
	return result, total, nil
}
func (r *Repository) GetVisibleThread(c context.Context, course, id string) (community.Thread, error) {
	var t community.Thread
	e := r.pool.QueryRow(c, `SELECT id,course_id,title,created_by_user_id,state,created_at,updated_at FROM community.thread WHERE id=$1 AND course_id=$2 AND state='VISIBLE'`, id, course).Scan(&t.ID, &t.CourseID, &t.Title, &t.CreatedByUserID, &t.State, &t.CreatedAt, &t.UpdatedAt)
	return t, err(e)
}
func (r *Repository) ListVisiblePosts(c context.Context, thread string, limit, offset int) ([]community.Post, int, error) {
	var total int
	if e := r.pool.QueryRow(c, `SELECT count(*) FROM community.post WHERE thread_id=$1 AND state='VISIBLE'`, thread).Scan(&total); e != nil {
		return nil, 0, err(e)
	}
	rows, e := r.pool.Query(c, `SELECT id,thread_id,author_user_id,body,state,created_at,updated_at FROM community.post WHERE thread_id=$1 AND state='VISIBLE' ORDER BY created_at ASC,id ASC LIMIT $2 OFFSET $3`, thread, limit, offset)
	if e != nil {
		return nil, 0, err(e)
	}
	defer rows.Close()
	result := []community.Post{}
	for rows.Next() {
		var p community.Post
		if e := rows.Scan(&p.ID, &p.ThreadID, &p.AuthorUserID, &p.Body, &p.State, &p.CreatedAt, &p.UpdatedAt); e != nil {
			return nil, 0, err(e)
		}
		result = append(result, p)
	}
	if e := rows.Err(); e != nil {
		return nil, 0, err(e)
	}
	return result, total, nil
}
func (r *Repository) CreatePost(c context.Context, in community.PostInput) (community.Post, error) {
	if in.Validate() != nil {
		return community.Post{}, community.ErrInvalid
	}
	body, _ := community.NormalizeBody(in.Body)
	var p community.Post
	e := r.pool.QueryRow(c, `WITH active_thread AS (UPDATE community.thread SET updated_at=now() WHERE id=$1 AND course_id=$2 AND state='VISIBLE' RETURNING id) INSERT INTO community.post(thread_id,author_user_id,body) SELECT id,$3,$4 FROM active_thread RETURNING id,thread_id,author_user_id,body,state,created_at,updated_at`, in.ThreadID, in.CourseID, in.AuthorUserID, body).Scan(&p.ID, &p.ThreadID, &p.AuthorUserID, &p.Body, &p.State, &p.CreatedAt, &p.UpdatedAt)
	return p, err(e)
}
func (r *Repository) SetThreadState(c context.Context, course, id string, state community.Visibility) (community.Thread, error) {
	var t community.Thread
	e := r.pool.QueryRow(c, `UPDATE community.thread SET state=$3 WHERE course_id=$1 AND id=$2 RETURNING id,course_id,title,created_by_user_id,state,created_at,updated_at`, course, id, state).Scan(&t.ID, &t.CourseID, &t.Title, &t.CreatedByUserID, &t.State, &t.CreatedAt, &t.UpdatedAt)
	return t, err(e)
}
func (r *Repository) SetPostState(c context.Context, course, thread, id string, state community.Visibility) (community.Post, error) {
	var p community.Post
	e := r.pool.QueryRow(c, `UPDATE community.post p SET state=$4 FROM community.thread t WHERE p.id=$3 AND p.thread_id=$2 AND t.id=p.thread_id AND t.course_id=$1 RETURNING p.id,p.thread_id,p.author_user_id,p.body,p.state,p.created_at,p.updated_at`, course, thread, id, state).Scan(&p.ID, &p.ThreadID, &p.AuthorUserID, &p.Body, &p.State, &p.CreatedAt, &p.UpdatedAt)
	return p, err(e)
}
func (r *Repository) ListThreadsForModeration(c context.Context, course string, limit, offset int) ([]community.ThreadSummary, int, error) {
	var n int
	if e := r.pool.QueryRow(c, `SELECT count(*) FROM community.thread WHERE course_id=$1`, course).Scan(&n); e != nil {
		return nil, 0, err(e)
	}
	rows, e := r.pool.Query(c, `SELECT t.id,t.course_id,t.title,t.created_by_user_id,t.state,t.created_at,t.updated_at,count(p.id) FROM community.thread t LEFT JOIN community.post p ON p.thread_id=t.id WHERE t.course_id=$1 GROUP BY t.id ORDER BY t.updated_at DESC,t.id DESC LIMIT $2 OFFSET $3`, course, limit, offset)
	if e != nil {
		return nil, 0, err(e)
	}
	defer rows.Close()
	out := []community.ThreadSummary{}
	for rows.Next() {
		var x community.ThreadSummary
		if e := rows.Scan(&x.ID, &x.CourseID, &x.Title, &x.CreatedByUserID, &x.State, &x.CreatedAt, &x.UpdatedAt, &x.PostCount); e != nil {
			return nil, 0, err(e)
		}
		out = append(out, x)
	}
	return out, n, rows.Err()
}
func (r *Repository) GetThreadForModeration(c context.Context, course, id string) (community.Thread, error) {
	var t community.Thread
	e := r.pool.QueryRow(c, `SELECT id,course_id,title,created_by_user_id,state,created_at,updated_at FROM community.thread WHERE course_id=$1 AND id=$2`, course, id).Scan(&t.ID, &t.CourseID, &t.Title, &t.CreatedByUserID, &t.State, &t.CreatedAt, &t.UpdatedAt)
	return t, err(e)
}
func (r *Repository) ListPostsForModeration(c context.Context, thread string, limit, offset int) ([]community.Post, int, error) {
	var n int
	if e := r.pool.QueryRow(c, `SELECT count(*) FROM community.post WHERE thread_id=$1`, thread).Scan(&n); e != nil {
		return nil, 0, err(e)
	}
	rows, e := r.pool.Query(c, `SELECT id,thread_id,author_user_id,body,state,created_at,updated_at FROM community.post WHERE thread_id=$1 ORDER BY created_at,id LIMIT $2 OFFSET $3`, thread, limit, offset)
	if e != nil {
		return nil, 0, err(e)
	}
	defer rows.Close()
	out := []community.Post{}
	for rows.Next() {
		var p community.Post
		if e := rows.Scan(&p.ID, &p.ThreadID, &p.AuthorUserID, &p.Body, &p.State, &p.CreatedAt, &p.UpdatedAt); e != nil {
			return nil, 0, err(e)
		}
		out = append(out, p)
	}
	return out, n, rows.Err()
}
func err(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return community.ErrNotFound
	}
	return e
}
