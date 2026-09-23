package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ credentials.Repository = (*Repository)(nil)

func (r *Repository) Create(ctx context.Context, input credentials.CertificateInput, issuedAt time.Time) (credentials.Certificate, error) {
	if r == nil || r.pool == nil || input.Validate() != nil || issuedAt.IsZero() {
		return credentials.Certificate{}, credentials.ErrInvalidCertificate
	}
	issuedAt = issuedAt.UTC()
	certificate, err := scan(r.pool.QueryRow(ctx, `INSERT INTO credentials.certificate (learner_user_id, course_id, course_version_id, course_title, course_version, language, criteria, issuer_id, issuer_name, issued_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        ON CONFLICT (learner_user_id, course_version_id) DO NOTHING
        RETURNING id, learner_user_id, course_id, course_version_id, course_title, course_version, language, criteria, issuer_id, issuer_name, issued_at, status, revoked_at`,
		input.LearnerUserID, input.CourseID, input.CourseVersionID, input.Achievement.CourseTitle, input.Achievement.CourseVersion, input.Achievement.Language, input.Achievement.Criteria, input.Issuer.ID, input.Issuer.Name, issuedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return r.GetByLearnerAndCourseVersion(ctx, input.LearnerUserID, input.CourseVersionID)
	}
	if err != nil {
		return credentials.Certificate{}, storage(err)
	}
	return certificate, nil
}

func (r *Repository) GetByID(ctx context.Context, id credentials.CertificateID) (credentials.Certificate, error) {
	if r == nil || r.pool == nil || !validUUID(string(id)) {
		return credentials.Certificate{}, credentials.ErrInvalidCertificate
	}
	certificate, err := scan(r.pool.QueryRow(ctx, `SELECT id, learner_user_id, course_id, course_version_id, course_title, course_version, language, criteria, issuer_id, issuer_name, issued_at, status, revoked_at FROM credentials.certificate WHERE id=$1`, id))
	if err != nil {
		return credentials.Certificate{}, storage(err)
	}
	return certificate, nil
}

func (r *Repository) GetByLearnerAndCourseVersion(ctx context.Context, learnerID, courseVersionID string) (credentials.Certificate, error) {
	if r == nil || r.pool == nil || !validUUID(learnerID) || !validUUID(courseVersionID) {
		return credentials.Certificate{}, credentials.ErrInvalidCertificate
	}
	certificate, err := scan(r.pool.QueryRow(ctx, `SELECT id, learner_user_id, course_id, course_version_id, course_title, course_version, language, criteria, issuer_id, issuer_name, issued_at, status, revoked_at FROM credentials.certificate WHERE learner_user_id=$1 AND course_version_id=$2`, learnerID, courseVersionID))
	if err != nil {
		return credentials.Certificate{}, storage(err)
	}
	return certificate, nil
}

func (r *Repository) Revoke(ctx context.Context, id credentials.CertificateID, revokedAt time.Time) (credentials.Certificate, error) {
	if r == nil || r.pool == nil || !validUUID(string(id)) || revokedAt.IsZero() {
		return credentials.Certificate{}, credentials.ErrInvalidCertificate
	}
	revokedAt = revokedAt.UTC()
	certificate, err := scan(r.pool.QueryRow(ctx, `UPDATE credentials.certificate SET status='REVOKED', revoked_at=$2 WHERE id=$1 AND status='ACTIVE' AND issued_at <= $2
        RETURNING id, learner_user_id, course_id, course_version_id, course_title, course_version, language, criteria, issuer_id, issuer_name, issued_at, status, revoked_at`, id, revokedAt))
	if err == nil {
		return certificate, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return credentials.Certificate{}, storage(err)
	}
	existing, getErr := r.GetByID(ctx, id)
	if getErr != nil {
		return credentials.Certificate{}, getErr
	}
	if existing.Status == credentials.CertificateRevoked {
		return existing, nil
	}
	return credentials.Certificate{}, credentials.ErrInvalidCertificate
}

type rowScanner interface{ Scan(...any) error }

func scan(row rowScanner) (credentials.Certificate, error) {
	var certificate credentials.Certificate
	var revokedAt *time.Time
	err := row.Scan(&certificate.ID, &certificate.LearnerUserID, &certificate.CourseID, &certificate.CourseVersionID, &certificate.Achievement.CourseTitle, &certificate.Achievement.CourseVersion, &certificate.Achievement.Language, &certificate.Achievement.Criteria, &certificate.Issuer.ID, &certificate.Issuer.Name, &certificate.IssuedAt, &certificate.Status, &revokedAt)
	if err != nil {
		return credentials.Certificate{}, err
	}
	certificate.IssuedAt = certificate.IssuedAt.UTC()
	if revokedAt != nil {
		at := revokedAt.UTC()
		certificate.RevokedAt = &at
	}
	if err := certificate.Validate(); err != nil {
		return credentials.Certificate{}, err
	}
	return certificate, nil
}

func storage(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return credentials.ErrCertificateNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return credentials.ErrInvalidCertificate
		case "23505":
			return credentials.ErrCertificateConflict
		}
	}
	return err
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return false
		}
	}
	return true
}
