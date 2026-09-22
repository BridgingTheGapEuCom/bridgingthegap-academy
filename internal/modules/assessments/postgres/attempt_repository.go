package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ assessments.AttemptRepository = (*Repository)(nil)

func (r *Repository) CreateAssessmentAttempt(ctx context.Context, input assessments.AttemptInput) (assessments.AssessmentAttempt, error) {
	canonical, err := assessments.CanonicalAttemptResponses(input.Responses)
	if err != nil {
		return assessments.AssessmentAttempt{}, err
	}
	input.Responses = canonical
	if err := input.Validate(); err != nil {
		return assessments.AssessmentAttempt{}, err
	}
	learner, err := uuid(input.LearnerUserID)
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	version, err := uuid(input.CourseVersionID)
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	assessment, err := uuid(string(input.AssessmentKey))
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	responses, err := marshalAttemptResponses(input.Responses)
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	row, err := r.q.CreateAssessmentAttempt(ctx, sqlc.CreateAssessmentAttemptParams{
		LearnerUserID: learner, CourseVersionID: version, AssessmentKey: assessment, Responses: responses,
	})
	if err != nil {
		return assessments.AssessmentAttempt{}, attemptStorageError(err)
	}
	return mapAttempt(row)
}

func (r *Repository) GetAssessmentAttempt(ctx context.Context, id assessments.AttemptID) (assessments.AssessmentAttempt, error) {
	key, err := uuid(string(id))
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	row, err := r.q.GetAssessmentAttempt(ctx, key)
	if err != nil {
		return assessments.AssessmentAttempt{}, attemptStorageError(err)
	}
	return mapAttempt(row)
}

func (r *Repository) UpdateAssessmentAttempt(ctx context.Context, id assessments.AttemptID, expectedRevision int64, update assessments.AttemptUpdate) (assessments.AssessmentAttempt, error) {
	if expectedRevision < 1 {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	canonical, err := assessments.CanonicalAttemptResponses(update.Responses)
	if err != nil {
		return assessments.AssessmentAttempt{}, err
	}
	update.Responses = canonical
	if err := update.Validate(); err != nil {
		return assessments.AssessmentAttempt{}, err
	}
	key, err := uuid(string(id))
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	responses, err := marshalAttemptResponses(update.Responses)
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	row, err := r.q.UpdateAssessmentAttempt(ctx, sqlc.UpdateAssessmentAttemptParams{ID: key, Revision: expectedRevision, Responses: responses})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.attemptUpdateMiss(ctx, id)
	}
	if err != nil {
		return assessments.AssessmentAttempt{}, attemptStorageError(err)
	}
	return mapAttempt(row)
}

func (r *Repository) SubmitAssessmentAttempt(ctx context.Context, id assessments.AttemptID, expectedRevision int64, submittedAt time.Time) (assessments.AssessmentAttempt, error) {
	if expectedRevision < 1 || submittedAt.IsZero() {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	key, err := uuid(string(id))
	if err != nil {
		return assessments.AssessmentAttempt{}, assessments.ErrInvalidAttempt
	}
	row, err := r.q.SubmitAssessmentAttempt(ctx, sqlc.SubmitAssessmentAttemptParams{ID: key, Revision: expectedRevision, SubmittedAt: pgtype.Timestamptz{Time: submittedAt.UTC(), Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.attemptUpdateMiss(ctx, id)
	}
	if err != nil {
		return assessments.AssessmentAttempt{}, attemptStorageError(err)
	}
	return mapAttempt(row)
}

func (r *Repository) attemptUpdateMiss(ctx context.Context, id assessments.AttemptID) (assessments.AssessmentAttempt, error) {
	attempt, err := r.GetAssessmentAttempt(ctx, id)
	if errors.Is(err, assessments.ErrAttemptNotFound) {
		return assessments.AssessmentAttempt{}, err
	}
	if err != nil {
		return assessments.AssessmentAttempt{}, err
	}
	if attempt.State == assessments.AttemptSubmitted {
		return assessments.AssessmentAttempt{}, assessments.ErrAttemptImmutable
	}
	return assessments.AssessmentAttempt{}, assessments.ErrRevisionMismatch
}

type persistedAttemptResponses struct {
	Responses []persistedAttemptResponse `json:"responses"`
}

type persistedAttemptResponse struct {
	QuestionKey        string                            `json:"questionKey"`
	Type               assessments.QuestionType          `json:"type"`
	SelectedOptionKey  string                            `json:"selectedOptionKey,omitempty"`
	SelectedOptionKeys []string                          `json:"selectedOptionKeys,omitempty"`
	Pairs              []assessments.AttemptMatchingPair `json:"pairs,omitempty"`
}

func marshalAttemptResponses(responses []assessments.AttemptResponse) ([]byte, error) {
	definition := persistedAttemptResponses{Responses: make([]persistedAttemptResponse, 0, len(responses))}
	for _, response := range responses {
		definition.Responses = append(definition.Responses, persistedAttemptResponse{
			QuestionKey: response.QuestionKey, Type: response.Type, SelectedOptionKey: response.SelectedOptionKey,
			SelectedOptionKeys: append([]string(nil), response.SelectedOptionKeys...), Pairs: append([]assessments.AttemptMatchingPair(nil), response.Pairs...),
		})
	}
	return json.Marshal(definition)
}

func parseAttemptResponses(value []byte) ([]assessments.AttemptResponse, error) {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	var definition persistedAttemptResponses
	if err := decoder.Decode(&definition); err != nil || definition.Responses == nil {
		return nil, assessments.ErrInvalidAttempt
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, assessments.ErrInvalidAttempt
	}
	responses := make([]assessments.AttemptResponse, 0, len(definition.Responses))
	for _, response := range definition.Responses {
		responses = append(responses, assessments.AttemptResponse{
			QuestionKey: response.QuestionKey, Type: response.Type, SelectedOptionKey: response.SelectedOptionKey,
			SelectedOptionKeys: append([]string(nil), response.SelectedOptionKeys...), Pairs: append([]assessments.AttemptMatchingPair(nil), response.Pairs...),
		})
	}
	if _, err := assessments.CanonicalAttemptResponses(responses); err != nil {
		return nil, err
	}
	return responses, nil
}

func mapAttempt(row sqlc.AssessmentsAssessmentAttempt) (assessments.AssessmentAttempt, error) {
	responses, err := parseAttemptResponses(row.Responses)
	if err != nil {
		return assessments.AssessmentAttempt{}, err
	}
	attempt := assessments.AssessmentAttempt{
		ID: assessments.AttemptID(row.ID.String()), LearnerUserID: row.LearnerUserID.String(), CourseVersionID: row.CourseVersionID.String(),
		AssessmentKey: assessments.AssessmentID(row.AssessmentKey.String()), State: assessments.AttemptState(row.State), Revision: row.Revision,
		Responses: responses, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
	if row.SubmittedAt.Valid {
		at := row.SubmittedAt.Time.UTC()
		attempt.SubmittedAt = &at
	}
	if err := attempt.Validate(); err != nil {
		return assessments.AssessmentAttempt{}, err
	}
	return attempt, nil
}

func attemptStorageError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return assessments.ErrAttemptNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return assessments.ErrAttemptConflict
		case "23502", "23503", "23514", "22P02":
			return assessments.ErrInvalidAttempt
		}
	}
	return errors.New("assessment attempt storage failure")
}
