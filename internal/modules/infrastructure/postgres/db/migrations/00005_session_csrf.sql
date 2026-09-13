-- +goose Up
-- Existing sessions are provisioned on their next authenticated session GET.
ALTER TABLE identity.sessions
    ADD COLUMN csrf_token text,
    ADD CONSTRAINT sessions_csrf_token_format CHECK
        (csrf_token IS NULL OR csrf_token ~ '^[A-Za-z0-9_-]{43}$');

COMMENT ON COLUMN identity.sessions.csrf_token IS
    'Sensitive session-bound synchronizer token; never log or expose outside CSRF delivery/validation';
