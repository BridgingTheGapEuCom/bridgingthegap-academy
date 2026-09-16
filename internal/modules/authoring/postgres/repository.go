package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/db/sqlc"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var _ authoring.Repository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool, q: sqlc.New(pool)} }

func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, errors.New("invalid authoring identifier")
	}
	return id, nil
}

func storageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.ErrNotFound
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return authoring.ErrConflict
		case "23503":
			return authoring.ErrNotFound
		case "23514", "23502", "22P02":
			return errors.New("authoring constraint violation")
		}
		return fmt.Errorf("authoring storage failure (SQLSTATE %s)", pgErr.Code)
	}
	return errors.New("authoring storage failure")
}

func encodeMetadata(m authoring.DraftMetadata) (sqlc.CreateDraftParams, error) {
	if err := m.Validate(); err != nil {
		return sqlc.CreateDraftParams{}, err
	}
	language, err := courses.NormalizeLanguageTag(string(m.SourceLanguage))
	if err != nil {
		return sqlc.CreateDraftParams{}, err
	}
	id, err := uuid(string(m.CourseID))
	if err != nil {
		return sqlc.CreateDraftParams{}, err
	}
	objectives, err := json.Marshal(m.LearningObjectives)
	if err != nil {
		return sqlc.CreateDraftParams{}, err
	}
	license, err := json.Marshal(m.License)
	if err != nil {
		return sqlc.CreateDraftParams{}, err
	}
	return sqlc.CreateDraftParams{CourseID: id, IntendedVersion: m.IntendedVersion.String(), SourceLanguage: string(language), Title: m.Title, Description: m.Description, LearningObjectives: objectives, Changelog: m.Changelog, License: license}, nil
}

func mapDraft(row sqlc.AuthoringCourseDraft) (authoring.CourseDraft, error) {
	version, err := courses.ParseVersion(row.IntendedVersion)
	if err != nil {
		return authoring.CourseDraft{}, errors.New("invalid stored draft version")
	}
	var objectives []string
	if err := json.Unmarshal(row.LearningObjectives, &objectives); err != nil {
		return authoring.CourseDraft{}, errors.New("invalid stored draft objectives")
	}
	var license courses.ContentLicense
	if err := json.Unmarshal(row.License, &license); err != nil {
		return authoring.CourseDraft{}, errors.New("invalid stored draft license")
	}
	metadata := authoring.DraftMetadata{CourseID: courses.CourseID(row.CourseID.String()), IntendedVersion: version, SourceLanguage: courses.LanguageTag(row.SourceLanguage), Title: row.Title, Description: row.Description, LearningObjectives: objectives, Changelog: row.Changelog, License: license}
	if err := metadata.Validate(); err != nil {
		return authoring.CourseDraft{}, errors.New("invalid stored draft metadata")
	}
	status := authoring.DraftStatus(row.Status)
	if !status.Valid() {
		return authoring.CourseDraft{}, errors.New("invalid stored draft status")
	}
	return authoring.CourseDraft{ID: authoring.DraftID(row.ID.String()), Metadata: metadata, Status: status, Revision: row.Revision, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, nil
}

func mapWorkspace(row sqlc.AuthoringWorkspace) authoring.AuthoringWorkspace {
	return authoring.AuthoringWorkspace{ID: authoring.WorkspaceID(row.ID.String()), DraftID: authoring.DraftID(row.DraftID.String()), CreatedByUserID: row.CreatedByUserID.String(), CreatedAt: row.CreatedAt.Time, LastActivityAt: row.LastActivityAt.Time}
}

func mapMember(row sqlc.AuthoringWorkspaceMember) authoring.WorkspaceMember {
	m := authoring.WorkspaceMember{ID: row.ID.String(), WorkspaceID: authoring.WorkspaceID(row.WorkspaceID.String()), UserID: row.UserID.String(), Role: authoring.MemberRole(row.Role), CreatedAt: row.CreatedAt.Time}
	if row.RevokedAt.Valid {
		v := row.RevokedAt.Time
		m.RevokedAt = &v
	}
	return m
}

func mapModule(row sqlc.AuthoringModule) authoring.DraftModule {
	return authoring.DraftModule{ID: authoring.ModuleID(row.ID.String()), ModuleInput: authoring.ModuleInput{DraftID: authoring.DraftID(row.DraftID.String()), StableKey: row.StableKey, Title: row.Title, Description: row.Description, Position: int(row.Position)}, Revision: row.Revision, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}

func mapLesson(row sqlc.AuthoringLesson) (authoring.DraftLesson, error) {
	var objectives []string
	if err := json.Unmarshal(row.LearningObjectives, &objectives); err != nil {
		return authoring.DraftLesson{}, errors.New("invalid stored lesson objectives")
	}
	content, err := courses.ParseLessonContent(row.Content)
	if err != nil {
		return authoring.DraftLesson{}, errors.New("invalid stored lesson content")
	}
	var duration *int
	if row.EstimatedDurationMinutes.Valid {
		v := int(row.EstimatedDurationMinutes.Int32)
		duration = &v
	}
	input := authoring.LessonInput{DraftID: authoring.DraftID(row.DraftID.String()), ModuleID: authoring.ModuleID(row.ModuleID.String()), StableKey: row.StableKey, Title: row.Title, Description: row.Description, LearningObjectives: objectives, EstimatedDurationMinutes: duration, Position: int(row.Position), Content: content}
	if err := input.Validate(); err != nil {
		return authoring.DraftLesson{}, errors.New("invalid stored lesson")
	}
	return authoring.DraftLesson{ID: authoring.LessonID(row.ID.String()), LessonInput: input, Revision: row.Revision, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, nil
}

func duration(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}

// A failed compare-and-swap is distinguished from absence and abandonment.
func (r *Repository) draftMiss(ctx context.Context, id pgtype.UUID) error {
	row, err := r.q.GetDraft(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.ErrNotFound
	}
	if err != nil {
		return storageError(err)
	}
	if row.Status != string(authoring.DraftActive) {
		return authoring.ErrInvalidState
	}
	return authoring.ErrRevisionMismatch
}
func (r *Repository) moduleMiss(ctx context.Context, id pgtype.UUID) error {
	row, err := r.q.GetModule(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.ErrNotFound
	}
	if err != nil {
		return storageError(err)
	}
	return r.draftMiss(ctx, row.DraftID)
}
func (r *Repository) lessonMiss(ctx context.Context, id pgtype.UUID) error {
	row, err := r.q.GetLesson(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.ErrNotFound
	}
	if err != nil {
		return storageError(err)
	}
	return r.draftMiss(ctx, row.DraftID)
}

func (r *Repository) CreateDraft(ctx context.Context, metadata authoring.DraftMetadata, createdBy string) (authoring.CourseDraft, authoring.AuthoringWorkspace, error) {
	args, err := encodeMetadata(metadata)
	if err != nil {
		return authoring.CourseDraft{}, authoring.AuthoringWorkspace{}, err
	}
	userID, err := uuid(createdBy)
	if err != nil {
		return authoring.CourseDraft{}, authoring.AuthoringWorkspace{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.CourseDraft{}, authoring.AuthoringWorkspace{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	row, err := q.CreateDraft(ctx, args)
	if err != nil {
		return authoring.CourseDraft{}, authoring.AuthoringWorkspace{}, storageError(err)
	}
	w, err := q.CreateWorkspace(ctx, sqlc.CreateWorkspaceParams{DraftID: row.ID, CreatedByUserID: userID})
	if err != nil {
		return authoring.CourseDraft{}, authoring.AuthoringWorkspace{}, storageError(err)
	}
	if _, err := q.AddMember(ctx, sqlc.AddMemberParams{WorkspaceID: w.ID, UserID: userID, Role: string(authoring.MemberMaintainer)}); err != nil {
		return authoring.CourseDraft{}, authoring.AuthoringWorkspace{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.CourseDraft{}, authoring.AuthoringWorkspace{}, storageError(err)
	}
	draft, err := mapDraft(row)
	return draft, mapWorkspace(w), err
}

func (r *Repository) GetDraft(ctx context.Context, id authoring.DraftID) (authoring.CourseDraft, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	row, err := r.q.GetDraft(ctx, key)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	return mapDraft(row)
}

// ListAccessibleDrafts is actor-scoped discovery. The active-membership join
// is the access boundary: no Identity lookup or role bypass is involved.
func (r *Repository) ListAccessibleDrafts(ctx context.Context, user string) ([]authoring.DraftSummary, error) {
	userID, err := uuid(user)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListAccessibleDrafts(ctx, userID)
	if err != nil {
		return nil, storageError(err)
	}
	result := make([]authoring.DraftSummary, 0, len(rows))
	for _, row := range rows {
		draft, err := mapDraft(row)
		if err != nil {
			return nil, err
		}
		result = append(result, authoring.DraftSummary{
			ID: draft.ID, Title: draft.Metadata.Title, IntendedVersion: draft.Metadata.IntendedVersion,
			Status: draft.Status, UpdatedAt: draft.UpdatedAt,
		})
	}
	return result, nil
}

func (r *Repository) GetWorkspace(ctx context.Context, id authoring.DraftID) (authoring.AuthoringWorkspace, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.AuthoringWorkspace{}, err
	}
	row, err := r.q.GetWorkspace(ctx, key)
	if err != nil {
		return authoring.AuthoringWorkspace{}, storageError(err)
	}
	return mapWorkspace(row), nil
}

func (r *Repository) UpdateDraftMetadata(ctx context.Context, id authoring.DraftID, expected int64, metadata authoring.DraftMetadata) (authoring.CourseDraft, error) {
	args, err := encodeMetadata(metadata)
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	current, err := r.GetDraft(ctx, id)
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	if current.Metadata.CourseID != metadata.CourseID {
		return authoring.CourseDraft{}, authoring.ErrConflict
	}
	key, err := uuid(string(id))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	row, err := q.UpdateDraftMetadata(ctx, sqlc.UpdateDraftMetadataParams{ID: key, Revision: expected, IntendedVersion: args.IntendedVersion, SourceLanguage: args.SourceLanguage, Title: args.Title, Description: args.Description, LearningObjectives: args.LearningObjectives, Changelog: args.Changelog, License: args.License})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.CourseDraft{}, r.draftMiss(ctx, key)
	}
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, key); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	return mapDraft(row)
}

func (r *Repository) AbandonDraft(ctx context.Context, id authoring.DraftID, expected int64) (authoring.CourseDraft, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.CourseDraft{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	row, err := q.AbandonDraft(ctx, sqlc.AbandonDraftParams{ID: key, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.CourseDraft{}, r.draftMiss(ctx, key)
	}
	if err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, key); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.CourseDraft{}, storageError(err)
	}
	return mapDraft(row)
}

func (r *Repository) ListMembers(ctx context.Context, workspace authoring.WorkspaceID) ([]authoring.WorkspaceMember, error) {
	wid, err := uuid(string(workspace))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListMembers(ctx, wid)
	if err != nil {
		return nil, storageError(err)
	}
	out := make([]authoring.WorkspaceMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapMember(row))
	}
	return out, nil
}

// ActiveMembers is deliberately draft-scoped. The HTTP read service has
// already authorized the draft and needs no workspace identifier from callers.
func (r *Repository) ActiveMembers(ctx context.Context, draft authoring.DraftID) ([]authoring.WorkspaceMember, error) {
	draftID, err := uuid(string(draft))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListActiveMembersForDraft(ctx, draftID)
	if err != nil {
		return nil, storageError(err)
	}
	result := make([]authoring.WorkspaceMember, 0, len(rows))
	for _, row := range rows {
		member := mapMember(row)
		if member.RevokedAt != nil {
			return nil, errors.New("active authoring member query returned revoked row")
		}
		result = append(result, member)
	}
	return result, nil
}

func (r *Repository) ActiveMembershipForDraft(ctx context.Context, draft authoring.DraftID, user string) (authoring.MemberRole, bool, error) {
	draftID, err := uuid(string(draft))
	if err != nil {
		return "", false, err
	}
	userID, err := uuid(user)
	if err != nil {
		return "", false, err
	}
	role, err := r.q.ActiveMembershipForDraft(ctx, sqlc.ActiveMembershipForDraftParams{DraftID: draftID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, storageError(err)
	}
	current := authoring.MemberRole(role)
	if !current.Valid() {
		return "", false, errors.New("invalid stored authoring membership")
	}
	return current, true, nil
}

func (r *Repository) CreateModule(ctx context.Context, input authoring.ModuleInput) (authoring.DraftModule, error) {
	if err := input.Validate(); err != nil {
		return authoring.DraftModule{}, err
	}
	id, err := uuid(string(input.DraftID))
	if err != nil {
		return authoring.DraftModule{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockActiveDraft(ctx, tx, id); err != nil {
		return authoring.DraftModule{}, err
	}
	q := r.q.WithTx(tx)
	row, err := q.CreateModule(ctx, sqlc.CreateModuleParams{ID: id, StableKey: input.StableKey, Title: input.Title, Description: input.Description, Position: int32(input.Position)})
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	if err := q.TouchDraft(ctx, id); err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	if err := q.TouchWorkspace(ctx, id); err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	return mapModule(row), nil
}

func (r *Repository) GetModule(ctx context.Context, id authoring.ModuleID) (authoring.DraftModule, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.DraftModule{}, err
	}
	row, err := r.q.GetModule(ctx, key)
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	return mapModule(row), nil
}

func (r *Repository) ListModules(ctx context.Context, id authoring.DraftID) ([]authoring.DraftModule, error) {
	key, err := uuid(string(id))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListModules(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	out := make([]authoring.DraftModule, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapModule(row))
	}
	return out, nil
}

func (r *Repository) UpdateModule(ctx context.Context, id authoring.ModuleID, expected int64, title, description string) (authoring.DraftModule, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.DraftModule{}, err
	}
	current, err := r.GetModule(ctx, id)
	if err != nil {
		return authoring.DraftModule{}, err
	}
	input := current.ModuleInput
	input.Title, input.Description = title, description
	if err := input.Validate(); err != nil {
		return authoring.DraftModule{}, err
	}
	draftID, err := uuid(string(current.DraftID))
	if err != nil {
		return authoring.DraftModule{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return authoring.DraftModule{}, err
	}
	q := r.q.WithTx(tx)
	row, err := q.UpdateModule(ctx, sqlc.UpdateModuleParams{ID: key, Revision: expected, Title: title, Description: description})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftModule{}, r.moduleMiss(ctx, key)
	}
	if err != nil {
		return authoring.DraftModule{}, storageError(err)
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

func (r *Repository) CreateLesson(ctx context.Context, input authoring.LessonInput) (authoring.DraftLesson, error) {
	if err := input.Validate(); err != nil {
		return authoring.DraftLesson{}, err
	}
	id, err := uuid(string(input.ModuleID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	draftID, err := uuid(string(input.DraftID))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	objectives, err := json.Marshal(input.LearningObjectives)
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	content, err := courses.MarshalLessonContent(input.Content)
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return authoring.DraftLesson{}, err
	}
	q := r.q.WithTx(tx)
	row, err := q.CreateLesson(ctx, sqlc.CreateLessonParams{ID: id, DraftID: draftID, StableKey: input.StableKey, Title: input.Title, Description: input.Description, LearningObjectives: objectives, EstimatedDurationMinutes: duration(input.EstimatedDurationMinutes), Position: int32(input.Position), Content: content})
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
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

func (r *Repository) GetLesson(ctx context.Context, id authoring.LessonID) (authoring.DraftLesson, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	row, err := r.q.GetLesson(ctx, key)
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
	}
	return mapLesson(row)
}

func (r *Repository) ListLessons(ctx context.Context, id authoring.ModuleID) ([]authoring.DraftLesson, error) {
	key, err := uuid(string(id))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListLessons(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	out := make([]authoring.DraftLesson, 0, len(rows))
	for _, row := range rows {
		lesson, err := mapLesson(row)
		if err != nil {
			return nil, err
		}
		out = append(out, lesson)
	}
	return out, nil
}

func (r *Repository) ListLessonsForDraft(ctx context.Context, id authoring.DraftID) ([]authoring.DraftLesson, error) {
	key, err := uuid(string(id))
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListLessonsForDraft(ctx, key)
	if err != nil {
		return nil, storageError(err)
	}
	out := make([]authoring.DraftLesson, 0, len(rows))
	for _, row := range rows {
		lesson, err := mapLesson(row)
		if err != nil {
			return nil, err
		}
		out = append(out, lesson)
	}
	return out, nil
}

func (r *Repository) UpdateLessonMetadata(ctx context.Context, id authoring.LessonID, expected int64, title, description string, objectives []string, estimated *int) (authoring.DraftLesson, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	current, err := r.GetLesson(ctx, id)
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	input := current.LessonInput
	input.Title, input.Description, input.LearningObjectives, input.EstimatedDurationMinutes = title, description, objectives, estimated
	if err := input.Validate(); err != nil {
		return authoring.DraftLesson{}, err
	}
	encoded, err := json.Marshal(objectives)
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
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return authoring.DraftLesson{}, err
	}
	q := r.q.WithTx(tx)
	row, err := q.UpdateLessonMetadata(ctx, sqlc.UpdateLessonMetadataParams{ID: key, Revision: expected, Title: title, Description: description, LearningObjectives: encoded, EstimatedDurationMinutes: duration(estimated)})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftLesson{}, r.lessonMiss(ctx, key)
	}
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
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

func (r *Repository) UpdateLessonContent(ctx context.Context, id authoring.LessonID, expected int64, content courses.LessonContent) (authoring.DraftLesson, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	encoded, err := courses.MarshalLessonContent(content)
	if err != nil {
		return authoring.DraftLesson{}, err
	}
	current, err := r.GetLesson(ctx, id)
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
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockActiveDraft(ctx, tx, draftID); err != nil {
		return authoring.DraftLesson{}, err
	}
	q := r.q.WithTx(tx)
	row, err := q.UpdateLessonContent(ctx, sqlc.UpdateLessonContentParams{ID: key, Revision: expected, Content: encoded})
	if errors.Is(err, pgx.ErrNoRows) {
		return authoring.DraftLesson{}, r.lessonMiss(ctx, key)
	}
	if err != nil {
		return authoring.DraftLesson{}, storageError(err)
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
