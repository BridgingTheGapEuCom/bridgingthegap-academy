package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assessments/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository struct{ q *sqlc.Queries }

var _ assessments.Repository = (*Repository)(nil)

func New(db sqlc.DBTX) *Repository { return &Repository{q: sqlc.New(db)} }

func (r *Repository) CreateAssessment(ctx context.Context, input assessments.AssessmentInput) (assessments.Assessment, error) {
	if input.Validate() != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	owner, err := uuid(input.OwnerDraftID)
	if err != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	creator, err := uuid(input.CreatedByUserID)
	if err != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	definition, err := marshalDefinition(input.Questions)
	if err != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	row, err := r.q.CreateAssessment(ctx, sqlc.CreateAssessmentParams{OwnerDraftID: owner, Title: input.Title, Definition: definition, CreatedByUserID: creator})
	if err != nil {
		return assessments.Assessment{}, storageError(err)
	}
	return mapAssessment(row)
}

func (r *Repository) GetAssessment(ctx context.Context, id assessments.AssessmentID) (assessments.Assessment, error) {
	key, err := uuid(string(id))
	if err != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	row, err := r.q.GetAssessment(ctx, key)
	if err != nil {
		return assessments.Assessment{}, storageError(err)
	}
	return mapAssessment(row)
}

func (r *Repository) UpdateAssessment(ctx context.Context, id assessments.AssessmentID, expectedRevision int64, update assessments.AssessmentUpdate) (assessments.Assessment, error) {
	if expectedRevision < 1 || update.Validate() != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	key, err := uuid(string(id))
	if err != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	definition, err := marshalDefinition(update.Questions)
	if err != nil {
		return assessments.Assessment{}, assessments.ErrInvalidAssessment
	}
	row, err := r.q.UpdateAssessment(ctx, sqlc.UpdateAssessmentParams{ID: key, Revision: expectedRevision, Title: update.Title, Definition: definition})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, getErr := r.GetAssessment(ctx, id); errors.Is(getErr, assessments.ErrAssessmentNotFound) {
			return assessments.Assessment{}, assessments.ErrAssessmentNotFound
		}
		return assessments.Assessment{}, assessments.ErrRevisionMismatch
	}
	if err != nil {
		return assessments.Assessment{}, storageError(err)
	}
	return mapAssessment(row)
}

func (r *Repository) ListAssessmentSummariesForDraft(ctx context.Context, draftID string, limit, offset int) ([]assessments.AssessmentSummary, int, error) {
	owner, err := uuid(draftID)
	if err != nil || limit < 1 || offset < 0 {
		return nil, 0, assessments.ErrInvalidAssessment
	}
	rows, err := r.q.ListAssessmentSummariesForDraft(ctx, sqlc.ListAssessmentSummariesForDraftParams{
		OwnerDraftID: owner, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, storageError(err)
	}
	total, err := r.q.CountAssessmentSummariesForDraft(ctx, owner)
	if err != nil {
		return nil, 0, storageError(err)
	}
	items := make([]assessments.AssessmentSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, assessments.AssessmentSummary{
			ID: assessments.AssessmentID(row.ID.String()), Title: row.Title, QuestionCount: int(row.QuestionCount), Revision: row.Revision, UpdatedAt: row.UpdatedAt.Time.UTC(),
		})
	}
	return items, int(total), nil
}

type persistedDefinition struct {
	Questions []persistedQuestion `json:"questions"`
}

type persistedQuestion struct {
	StableKey         string                     `json:"stableKey"`
	Type              assessments.QuestionType   `json:"type"`
	Prompt            string                     `json:"prompt"`
	Position          int                        `json:"position"`
	Options           []assessments.ChoiceOption `json:"options,omitempty"`
	CorrectOptionKeys []string                   `json:"correctOptionKeys,omitempty"`
	LeftItems         []assessments.MatchingItem `json:"leftItems,omitempty"`
	RightItems        []assessments.MatchingItem `json:"rightItems,omitempty"`
	CorrectPairs      []assessments.MatchingPair `json:"correctPairs,omitempty"`
}

func marshalDefinition(questions []assessments.Question) ([]byte, error) {
	definition := persistedDefinition{Questions: make([]persistedQuestion, 0, len(questions))}
	for _, question := range questions {
		definition.Questions = append(definition.Questions, persistedQuestion{
			StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
			Options:           append([]assessments.ChoiceOption(nil), question.Options...),
			CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...),
			LeftItems:         append([]assessments.MatchingItem(nil), question.LeftItems...),
			RightItems:        append([]assessments.MatchingItem(nil), question.RightItems...),
			CorrectPairs:      append([]assessments.MatchingPair(nil), question.CorrectPairs...),
		})
	}
	return json.Marshal(definition)
}

func parseDefinition(value []byte) ([]assessments.Question, error) {
	var definition persistedDefinition
	if err := json.Unmarshal(value, &definition); err != nil || definition.Questions == nil {
		return nil, assessments.ErrInvalidAssessment
	}
	questions := make([]assessments.Question, 0, len(definition.Questions))
	for _, question := range definition.Questions {
		questions = append(questions, assessments.Question{
			StableKey: question.StableKey, Type: question.Type, Prompt: question.Prompt, Position: question.Position,
			Options:           append([]assessments.ChoiceOption(nil), question.Options...),
			CorrectOptionKeys: append([]string(nil), question.CorrectOptionKeys...),
			LeftItems:         append([]assessments.MatchingItem(nil), question.LeftItems...),
			RightItems:        append([]assessments.MatchingItem(nil), question.RightItems...),
			CorrectPairs:      append([]assessments.MatchingPair(nil), question.CorrectPairs...),
		})
	}
	return questions, nil
}

func mapAssessment(row sqlc.AssessmentsAssessment) (assessments.Assessment, error) {
	questions, err := parseDefinition(row.Definition)
	if err != nil {
		return assessments.Assessment{}, err
	}
	assessment := assessments.Assessment{
		ID: assessments.AssessmentID(row.ID.String()), OwnerDraftID: row.OwnerDraftID.String(), Title: row.Title,
		Revision: row.Revision, Questions: questions, CreatedByUserID: row.CreatedByUserID.String(),
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
	if err := assessment.Validate(); err != nil {
		return assessments.Assessment{}, err
	}
	return assessment, nil
}

func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, assessments.ErrInvalidAssessment
	}
	return id, nil
}

func storageError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return assessments.ErrAssessmentNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return assessments.ErrAssessmentConflict
		case "23502", "23514", "22P02":
			return assessments.ErrInvalidAssessment
		}
	}
	return errors.New("assessment storage failure")
}
