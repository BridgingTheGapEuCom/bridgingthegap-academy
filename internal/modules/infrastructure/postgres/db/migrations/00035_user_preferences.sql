-- +goose Up
CREATE SCHEMA preferences;

CREATE TABLE preferences.user_preferences (
    user_id uuid PRIMARY KEY REFERENCES identity.users(id),
    application_locale text,
    appearance text NOT NULL DEFAULT 'SYSTEM' CHECK (appearance IN ('SYSTEM', 'LIGHT', 'DARK')),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_preferences_application_locale_trimmed CHECK (application_locale IS NULL OR application_locale = btrim(application_locale)),
    CONSTRAINT user_preferences_updated_after_created CHECK (updated_at >= created_at)
);

-- +goose Down
DROP SCHEMA preferences;
