package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/preferences"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/preferences/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository struct{ q *sqlc.Queries }

var _ preferences.Repository = (*Repository)(nil)

func New(db sqlc.DBTX) *Repository { return &Repository{q: sqlc.New(db)} }

func uuid(value identity.UserID) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(string(value)); err != nil || !id.Valid {
		return id, preferences.ErrInvalidPreferences
	}
	return id, nil
}

func locale(value *preferences.ApplicationLocale) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*value), Valid: true}
}

func optionalLocale(value pgtype.Text) *preferences.ApplicationLocale {
	if !value.Valid {
		return nil
	}
	locale := preferences.ApplicationLocale(value.String)
	return &locale
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func storageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return identity.ErrConflict
	}
	return errors.New("user preferences storage failure")
}

func mapPreferences(row sqlc.PreferencesUserPreference) (preferences.UserPreferences, error) {
	appearance, err := preferences.ParseAppearance(row.Appearance)
	if err != nil {
		return preferences.UserPreferences{}, err
	}
	if value := optionalLocale(row.ApplicationLocale); value != nil {
		if _, err := preferences.ParseApplicationLocale(string(*value)); err != nil {
			return preferences.UserPreferences{}, err
		}
	}
	return preferences.UserPreferences{UserID: identity.UserID(row.UserID.String()), ApplicationLocale: optionalLocale(row.ApplicationLocale), Appearance: appearance, Revision: row.Revision, CreatedAt: optionalTime(row.CreatedAt), UpdatedAt: optionalTime(row.UpdatedAt)}, nil
}

func (r *Repository) GetUserPreferences(ctx context.Context, userID identity.UserID) (preferences.UserPreferences, error) {
	id, err := uuid(userID)
	if err != nil {
		return preferences.UserPreferences{}, err
	}
	row, err := r.q.GetUserPreferences(ctx, id)
	if err != nil {
		return preferences.UserPreferences{}, storageError(err)
	}
	return mapPreferences(row)
}

func (r *Repository) CreateUserPreferences(ctx context.Context, userID identity.UserID, update preferences.Update) (preferences.UserPreferences, error) {
	id, err := uuid(userID)
	if err != nil {
		return preferences.UserPreferences{}, err
	}
	row, err := r.q.CreateUserPreferences(ctx, sqlc.CreateUserPreferencesParams{UserID: id, ApplicationLocale: locale(update.ApplicationLocale), Appearance: string(update.Appearance)})
	if err != nil {
		return preferences.UserPreferences{}, storageError(err)
	}
	return mapPreferences(row)
}

func (r *Repository) UpdateUserPreferences(ctx context.Context, userID identity.UserID, expectedRevision int64, update preferences.Update) (preferences.UserPreferences, error) {
	id, err := uuid(userID)
	if err != nil {
		return preferences.UserPreferences{}, err
	}
	row, err := r.q.UpdateUserPreferences(ctx, sqlc.UpdateUserPreferencesParams{UserID: id, ApplicationLocale: locale(update.ApplicationLocale), Appearance: string(update.Appearance), Revision: expectedRevision})
	if err != nil {
		return preferences.UserPreferences{}, storageError(err)
	}
	return mapPreferences(row)
}
