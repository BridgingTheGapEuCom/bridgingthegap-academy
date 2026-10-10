-- name: GetUserPreferences :one
SELECT user_id, application_locale, appearance, revision, created_at, updated_at
FROM preferences.user_preferences
WHERE user_id = $1;

-- name: CreateUserPreferences :one
INSERT INTO preferences.user_preferences (user_id, application_locale, appearance)
VALUES ($1, $2, $3)
RETURNING user_id, application_locale, appearance, revision, created_at, updated_at;

-- name: UpdateUserPreferences :one
UPDATE preferences.user_preferences
SET application_locale = $2, appearance = $3, revision = revision + 1, updated_at = now()
WHERE user_id = $1 AND revision = $4
RETURNING user_id, application_locale, appearance, revision, created_at, updated_at;
