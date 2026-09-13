-- +goose Up
CREATE SCHEMA identity;

CREATE TABLE identity.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_updated_after_created CHECK (updated_at >= created_at)
);

CREATE TABLE identity.user_emails (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES identity.users(id),
    normalized_email text NOT NULL,
    display_email text NOT NULL,
    verified_at timestamptz,
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_emails_normalized CHECK (normalized_email = lower(btrim(normalized_email)) AND normalized_email ~ '^[^[:space:]@]+@[^[:space:]@]+$'),
    CONSTRAINT user_emails_display_not_empty CHECK (display_email <> ''),
    CONSTRAINT user_emails_verified_after_created CHECK (verified_at IS NULL OR verified_at >= created_at)
);
CREATE UNIQUE INDEX user_emails_normalized_unique ON identity.user_emails (normalized_email);
CREATE UNIQUE INDEX user_emails_primary_per_user ON identity.user_emails (user_id) WHERE is_primary;
CREATE INDEX user_emails_user_id_idx ON identity.user_emails (user_id);

CREATE TABLE identity.local_password_credentials (
    user_id uuid PRIMARY KEY REFERENCES identity.users(id),
    password_hash text NOT NULL CHECK (password_hash <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT local_password_credentials_updated_after_created CHECK (updated_at >= created_at)
);
COMMENT ON COLUMN identity.local_password_credentials.password_hash IS 'Sensitive password verifier; never log or expose through unrelated queries';

CREATE TABLE identity.global_role_assignments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES identity.users(id),
    role text NOT NULL CHECK (role IN ('ADMINISTRATOR')),
    granted_at timestamptz NOT NULL DEFAULT now(),
    granted_by uuid REFERENCES identity.users(id),
    revoked_at timestamptz,
    CONSTRAINT global_role_assignments_revoked_after_granted CHECK (revoked_at IS NULL OR revoked_at >= granted_at)
);
CREATE UNIQUE INDEX global_role_assignments_active_unique ON identity.global_role_assignments (user_id, role) WHERE revoked_at IS NULL;
CREATE INDEX global_role_assignments_user_id_idx ON identity.global_role_assignments (user_id, granted_at DESC);

CREATE TABLE identity.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES identity.users(id),
    token_digest bytea NOT NULL CHECK (octet_length(token_digest) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CONSTRAINT sessions_expires_after_created CHECK (expires_at > created_at),
    CONSTRAINT sessions_seen_after_created CHECK (last_seen_at >= created_at),
    CONSTRAINT sessions_revoked_after_created CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE UNIQUE INDEX sessions_token_digest_unique ON identity.sessions (token_digest);
CREATE INDEX sessions_user_active_idx ON identity.sessions (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expires_at_idx ON identity.sessions (expires_at);
COMMENT ON COLUMN identity.sessions.token_digest IS 'SHA-256 digest of opaque session token; raw browser token must not be stored';
