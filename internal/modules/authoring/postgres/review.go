package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ authoring.ReviewRepository = (*Repository)(nil)

type reviewRow struct {
	id, draftID, submittedBy pgtype.UUID
	draftRevision            int64
	snapshotVersion          int32
	snapshot                 []byte
	status                   string
	revision                 int64
	submittedAt              pgtype.Timestamptz
	decidedBy                pgtype.UUID
	decidedAt                pgtype.Timestamptz
}

const reviewColumns = `id,draft_id,draft_revision,snapshot_schema_version,snapshot,status,revision,submitted_by_user_id,submitted_at,decided_by_user_id,decided_at`

func scanReview(row pgx.Row) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	var stored reviewRow
	if err := row.Scan(&stored.id, &stored.draftID, &stored.draftRevision, &stored.snapshotVersion, &stored.snapshot, &stored.status, &stored.revision, &stored.submittedBy, &stored.submittedAt, &stored.decidedBy, &stored.decidedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, authoring.ErrReviewNotFound
		}
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	var snapshot authoring.ReviewSnapshot
	if err := json.Unmarshal(stored.snapshot, &snapshot); err != nil || snapshot.Validate() != nil || snapshot.SchemaVersion != int(stored.snapshotVersion) || snapshot.Draft.Revision != stored.draftRevision || snapshot.Draft.ID != authoring.DraftID(stored.draftID.String()) {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, authoring.ErrReviewSnapshot
	}
	cycle := authoring.ReviewCycle{ID: authoring.ReviewID(stored.id.String()), DraftID: authoring.DraftID(stored.draftID.String()), DraftRevision: stored.draftRevision, SnapshotSchemaVersion: int(stored.snapshotVersion), Status: authoring.ReviewStatus(stored.status), Revision: stored.revision, SubmittedByUserID: stored.submittedBy.String(), SubmittedAt: stored.submittedAt.Time}
	if !cycle.Status.ValidCycleStatus() {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, authoring.ErrReviewSnapshot
	}
	if stored.decidedBy.Valid {
		cycle.DecidedByUserID = stored.decidedBy.String()
	}
	if stored.decidedAt.Valid {
		decided := stored.decidedAt.Time
		cycle.DecidedAt = &decided
	}
	return cycle, snapshot, nil
}

func (r *Repository) SubmitReview(ctx context.Context, draftID authoring.DraftID, expected int64, actor string) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	if draftID == "" || expected < 1 || authoring.ValidateReviewActor(actor) != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, authoring.ErrReviewSnapshot
	}
	draftKey, err := uuid(string(draftID))
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	actorKey, err := uuid(actor)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.lockActiveDraftRevision(ctx, tx, draftKey, expected); err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	q := r.q.WithTx(tx)
	draftRow, err := q.GetDraft(ctx, draftKey)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	draft, err := mapDraft(draftRow)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	moduleRows, err := q.ListModules(ctx, draftKey)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	lessonRows, err := q.ListLessonsForDraft(ctx, draftKey)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	prerequisiteRows, err := q.ListPrerequisitesForDraft(ctx, draftKey)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	modules := make([]authoring.DraftModule, 0, len(moduleRows))
	for _, row := range moduleRows {
		modules = append(modules, mapModule(row))
	}
	lessons := make([]authoring.DraftLesson, 0, len(lessonRows))
	for _, row := range lessonRows {
		lesson, mapErr := mapLesson(row)
		if mapErr != nil {
			return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, mapErr
		}
		lessons = append(lessons, lesson)
	}
	prerequisites := make([]authoring.Prerequisite, 0, len(prerequisiteRows))
	for _, row := range prerequisiteRows {
		prerequisites = append(prerequisites, authoring.Prerequisite{LessonID: authoring.LessonID(row.LessonID.String()), TargetLessonID: authoring.LessonID(row.PrerequisiteLessonID.String()), TargetStableKey: row.TargetStableKey, Position: int(row.Position)})
	}
	snapshot, err := authoring.NewReviewSnapshot(draft, modules, lessons, prerequisites)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	encoded, err := authoring.MarshalReviewSnapshot(snapshot)
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	row := tx.QueryRow(ctx, `INSERT INTO authoring.review_cycle(draft_id,draft_revision,snapshot_schema_version,snapshot,submitted_by_user_id) VALUES($1,$2,$3,$4,$5) RETURNING `+reviewColumns, draftKey, expected, snapshot.SchemaVersion, encoded, actorKey)
	cycle, stored, err := scanReview(row)
	if err != nil {
		if errors.Is(err, authoring.ErrConflict) {
			err = authoring.ErrReviewAlreadyExists
		}
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	reviewKey, _ := uuid(string(cycle.ID))
	if _, err := tx.Exec(ctx, `INSERT INTO authoring.review_event(review_id,event_type,actor_user_id) VALUES($1,'SUBMITTED',$2)`, reviewKey, actorKey); err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, storageError(err)
	}
	return cycle, stored, nil
}

func (r *Repository) GetReview(ctx context.Context, id authoring.ReviewID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	key, err := uuid(string(id))
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	return scanReview(r.pool.QueryRow(ctx, `SELECT `+reviewColumns+` FROM authoring.review_cycle WHERE id=$1`, key))
}

func (r *Repository) GetReviewForDraft(ctx context.Context, draft authoring.DraftID, id authoring.ReviewID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	draftKey, err := uuid(string(draft))
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	reviewKey, err := uuid(string(id))
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	return scanReview(r.pool.QueryRow(ctx, `SELECT `+reviewColumns+` FROM authoring.review_cycle WHERE id=$1 AND draft_id=$2`, reviewKey, draftKey))
}

func (r *Repository) LatestReview(ctx context.Context, draft authoring.DraftID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	key, err := uuid(string(draft))
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	return scanReview(r.pool.QueryRow(ctx, `SELECT `+reviewColumns+` FROM authoring.review_cycle WHERE draft_id=$1 ORDER BY submitted_at DESC,id DESC LIMIT 1`, key))
}

func (r *Repository) ActiveReview(ctx context.Context, draft authoring.DraftID) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	key, err := uuid(string(draft))
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	return scanReview(r.pool.QueryRow(ctx, `SELECT `+reviewColumns+` FROM authoring.review_cycle WHERE draft_id=$1 AND status='IN_REVIEW'`, key))
}

func (r *Repository) ApprovedReviewForRevision(ctx context.Context, draft authoring.DraftID, revision int64) (authoring.ReviewCycle, authoring.ReviewSnapshot, error) {
	key, err := uuid(string(draft))
	if err != nil {
		return authoring.ReviewCycle{}, authoring.ReviewSnapshot{}, err
	}
	return scanReview(r.pool.QueryRow(ctx, `SELECT `+reviewColumns+` FROM authoring.review_cycle WHERE draft_id=$1 AND draft_revision=$2 AND status='APPROVED'`, key, revision))
}

func (r *Repository) ListReviewHistory(ctx context.Context, draft authoring.DraftID) ([]authoring.ReviewCycle, error) {
	key, err := uuid(string(draft))
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+reviewColumns+` FROM authoring.review_cycle WHERE draft_id=$1 ORDER BY submitted_at DESC,id DESC`, key)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	result := []authoring.ReviewCycle{}
	for rows.Next() {
		cycle, _, scanErr := scanReview(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, cycle)
	}
	if rows.Err() != nil {
		return nil, storageError(rows.Err())
	}
	return result, nil
}

func (r *Repository) DecideReview(ctx context.Context, id authoring.ReviewID, expected int64, decision authoring.ReviewStatus, actor, message string) (authoring.ReviewCycle, error) {
	return r.decideReview(ctx, "", id, expected, decision, actor, message)
}

func (r *Repository) DecideReviewForDraft(ctx context.Context, draft authoring.DraftID, id authoring.ReviewID, expected int64, decision authoring.ReviewStatus, actor, message string) (authoring.ReviewCycle, error) {
	return r.decideReview(ctx, draft, id, expected, decision, actor, message)
}

func (r *Repository) decideReview(ctx context.Context, draft authoring.DraftID, id authoring.ReviewID, expected int64, decision authoring.ReviewStatus, actor, message string) (authoring.ReviewCycle, error) {
	if decision != authoring.ReviewApproved && decision != authoring.ReviewChangesRequested || expected < 1 || authoring.ValidateReviewActor(actor) != nil || authoring.ValidateReviewMessage(message) != nil {
		return authoring.ReviewCycle{}, authoring.ErrReviewInvalidState
	}
	reviewKey, err := uuid(string(id))
	if err != nil {
		return authoring.ReviewCycle{}, err
	}
	actorKey, err := uuid(actor)
	if err != nil {
		return authoring.ReviewCycle{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authoring.ReviewCycle{}, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	query := `SELECT ` + reviewColumns + ` FROM authoring.review_cycle WHERE id=$1`
	arguments := []any{reviewKey}
	if draft != "" {
		draftKey, draftErr := uuid(string(draft))
		if draftErr != nil {
			return authoring.ReviewCycle{}, draftErr
		}
		query += ` AND draft_id=$2`
		arguments = append(arguments, draftKey)
	}
	query += ` FOR UPDATE`
	cycle, _, err := scanReview(tx.QueryRow(ctx, query, arguments...))
	if err != nil {
		return authoring.ReviewCycle{}, err
	}
	if err := cycle.CanDecide(expected); err != nil {
		return authoring.ReviewCycle{}, err
	}
	row := tx.QueryRow(ctx, `UPDATE authoring.review_cycle SET status=$2,revision=revision+1,decided_by_user_id=$3,decided_at=now() WHERE id=$1 AND revision=$4 AND status='IN_REVIEW' RETURNING `+reviewColumns, reviewKey, string(decision), actorKey, expected)
	decided, _, err := scanReview(row)
	if err != nil {
		return authoring.ReviewCycle{}, err
	}
	eventType := authoring.ReviewApprovedEvent
	if decision == authoring.ReviewChangesRequested {
		eventType = authoring.ReviewChangesRequestedEvent
	}
	var eventMessage any
	if message != "" {
		eventMessage = message
	}
	if _, err := tx.Exec(ctx, `INSERT INTO authoring.review_event(review_id,event_type,actor_user_id,message) VALUES($1,$2,$3,$4)`, reviewKey, string(eventType), actorKey, eventMessage); err != nil {
		return authoring.ReviewCycle{}, storageError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return authoring.ReviewCycle{}, storageError(err)
	}
	return decided, nil
}

func (r *Repository) ListReviewEvents(ctx context.Context, id authoring.ReviewID) ([]authoring.ReviewEvent, error) {
	key, err := uuid(string(id))
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,event_type,actor_user_id,message,created_at FROM authoring.review_event WHERE review_id=$1 ORDER BY created_at,id`, key)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	result := []authoring.ReviewEvent{}
	for rows.Next() {
		var eventID, actorID pgtype.UUID
		var eventType string
		var message *string
		var created time.Time
		if err := rows.Scan(&eventID, &eventType, &actorID, &message, &created); err != nil {
			return nil, storageError(err)
		}
		result = append(result, authoring.ReviewEvent{ID: eventID.String(), ReviewID: id, Type: authoring.ReviewEventType(eventType), ActorUserID: actorID.String(), Message: valueOrEmpty(message), CreatedAt: created})
	}
	if rows.Err() != nil {
		return nil, storageError(rows.Err())
	}
	return result, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
