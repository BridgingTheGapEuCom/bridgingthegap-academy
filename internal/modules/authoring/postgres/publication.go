package postgres

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/authoring/db/sqlc"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ authoring.PublicationRecordRepository = (*Repository)(nil)

func (r *Repository) RecordPublication(ctx context.Context, record authoring.PublicationRecord) (authoring.PublicationRecord, error) {
	params, err := publicationParams(record)
	if err != nil {
		return authoring.PublicationRecord{}, err
	}
	row, err := r.q.CreateReviewPublication(ctx, params)
	if err == nil {
		return mapPublication(row)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := r.GetPublication(ctx, record.ReviewID)
		if getErr != nil {
			return authoring.PublicationRecord{}, getErr
		}
		if samePublicationRecord(existing, record) {
			return existing, nil
		}
		return authoring.PublicationRecord{}, authoring.ErrPublicationConflict
	}
	mapped := storageError(err)
	if errors.Is(mapped, authoring.ErrConflict) {
		return authoring.PublicationRecord{}, authoring.ErrPublicationConflict
	}
	return authoring.PublicationRecord{}, mapped
}

func (r *Repository) GetPublication(ctx context.Context, reviewID authoring.ReviewID) (authoring.PublicationRecord, error) {
	key, err := uuid(string(reviewID))
	if err != nil {
		return authoring.PublicationRecord{}, authoring.ErrPublicationInvalidInput
	}
	row, err := r.q.GetReviewPublication(ctx, key)
	if err != nil {
		return authoring.PublicationRecord{}, storageError(err)
	}
	return mapPublication(row)
}

func publicationParams(record authoring.PublicationRecord) (sqlc.CreateReviewPublicationParams, error) {
	if record.ReviewRevision < 1 || record.DraftRevision < 1 || record.CourseID == "" || record.CourseVersionID == "" || record.PublishedAt.IsZero() || authoring.ValidateReviewActor(record.PublishedByUserID) != nil {
		return sqlc.CreateReviewPublicationParams{}, authoring.ErrPublicationInvalidInput
	}
	reviewID, err := uuid(string(record.ReviewID))
	if err != nil {
		return sqlc.CreateReviewPublicationParams{}, authoring.ErrPublicationInvalidInput
	}
	draftID, err := uuid(string(record.DraftID))
	if err != nil {
		return sqlc.CreateReviewPublicationParams{}, authoring.ErrPublicationInvalidInput
	}
	courseID, err := uuid(string(record.CourseID))
	if err != nil {
		return sqlc.CreateReviewPublicationParams{}, authoring.ErrPublicationInvalidInput
	}
	versionID, err := uuid(string(record.CourseVersionID))
	if err != nil {
		return sqlc.CreateReviewPublicationParams{}, authoring.ErrPublicationInvalidInput
	}
	publisherID, err := uuid(record.PublishedByUserID)
	if err != nil {
		return sqlc.CreateReviewPublicationParams{}, authoring.ErrPublicationInvalidInput
	}
	if _, err := courses.ParseVersion(record.CourseVersion.String()); err != nil {
		return sqlc.CreateReviewPublicationParams{}, authoring.ErrPublicationInvalidInput
	}
	return sqlc.CreateReviewPublicationParams{
		ReviewID: reviewID, ReviewRevision: record.ReviewRevision,
		DraftID: draftID, DraftRevision: record.DraftRevision,
		CourseID: courseID, CourseVersion: record.CourseVersion.String(),
		CourseVersionID:   versionID,
		PublishedAt:       pgtype.Timestamptz{Time: record.PublishedAt, Valid: true},
		PublishedByUserID: publisherID,
	}, nil
}

func mapPublication(row sqlc.AuthoringReviewPublication) (authoring.PublicationRecord, error) {
	version, err := courses.ParseVersion(row.CourseVersion)
	if err != nil {
		return authoring.PublicationRecord{}, authoring.ErrPublicationInvalidInput
	}
	return authoring.PublicationRecord{
		ReviewID: authoring.ReviewID(row.ReviewID.String()), ReviewRevision: row.ReviewRevision,
		DraftID: authoring.DraftID(row.DraftID.String()), DraftRevision: row.DraftRevision,
		CourseID: courses.CourseID(row.CourseID.String()), CourseVersion: version,
		CourseVersionID: courses.CourseVersionID(row.CourseVersionID.String()),
		PublishedAt:     row.PublishedAt.Time.UTC(), PublishedByUserID: row.PublishedByUserID.String(),
		RecordedAt: row.RecordedAt.Time.UTC(),
	}, nil
}

func samePublicationRecord(left, right authoring.PublicationRecord) bool {
	return left.ReviewID == right.ReviewID && left.ReviewRevision == right.ReviewRevision &&
		left.DraftID == right.DraftID && left.DraftRevision == right.DraftRevision &&
		left.CourseID == right.CourseID && left.CourseVersion == right.CourseVersion &&
		left.CourseVersionID == right.CourseVersionID && left.PublishedAt.Equal(right.PublishedAt) &&
		left.PublishedByUserID == right.PublishedByUserID
}
