-- +goose Up
CREATE SCHEMA plugins;

CREATE TABLE plugins.verification_key (
    key_id text PRIMARY KEY,
    public_key bytea NOT NULL CHECK (octet_length(public_key) = 32),
    purpose text NOT NULL CHECK (purpose IN ('BTG_OWNED_SIGNING', 'BTG_APPROVAL_SIGNING')),
    allowed_plugin_ids text[] NOT NULL DEFAULT '{}',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((purpose = 'BTG_OWNED_SIGNING' AND cardinality(allowed_plugin_ids) > 0)
        OR (purpose = 'BTG_APPROVAL_SIGNING' AND cardinality(allowed_plugin_ids) = 0))
);

CREATE TABLE plugins.installed_release (
    installation_id uuid PRIMARY KEY,
    plugin_id text NOT NULL,
    version text NOT NULL,
    artifact_digest text NOT NULL CHECK (artifact_digest ~ '^[0-9a-f]{64}$'),
    manifest jsonb NOT NULL,
    signature_key_id text,
    signature_value text,
    signing_payload bytea,
    registered_trust text NOT NULL CHECK (registered_trust IN ('BTG_OWNED', 'BTG_APPROVED', 'UNKNOWN')),
    state text NOT NULL DEFAULT 'DISABLED' CHECK (state IN ('INSTALLED', 'DISABLED')),
    enabled boolean NOT NULL DEFAULT false,
    installed_at timestamptz NOT NULL,
    UNIQUE (plugin_id, version),
    UNIQUE (plugin_id, version, artifact_digest),
    CHECK ((signature_key_id IS NULL AND signature_value IS NULL AND signing_payload IS NULL)
        OR (signature_key_id IS NOT NULL AND signature_value IS NOT NULL AND signing_payload IS NOT NULL)),
    CHECK ((enabled AND state = 'INSTALLED') OR (NOT enabled AND state = 'DISABLED'))
);

CREATE TABLE plugins.release_approval (
    approval_id uuid PRIMARY KEY,
    plugin_id text NOT NULL,
    version text NOT NULL,
    artifact_digest text NOT NULL CHECK (artifact_digest ~ '^[0-9a-f]{64}$'),
    kind text NOT NULL CHECK (kind IN ('BTG_RELEASE_APPROVAL', 'LOCAL_UNKNOWN_APPROVAL')),
    authority_key_id text REFERENCES plugins.verification_key(key_id),
    approved_at timestamptz NOT NULL,
    revoked_at timestamptz,
    FOREIGN KEY (plugin_id, version, artifact_digest)
        REFERENCES plugins.installed_release(plugin_id, version, artifact_digest) ON DELETE RESTRICT,
    CHECK ((kind = 'BTG_RELEASE_APPROVAL' AND authority_key_id IS NOT NULL)
        OR (kind = 'LOCAL_UNKNOWN_APPROVAL' AND authority_key_id IS NULL)),
    CHECK (revoked_at IS NULL OR revoked_at >= approved_at)
);

CREATE UNIQUE INDEX release_approval_active_unique
ON plugins.release_approval (plugin_id, version, artifact_digest, kind, COALESCE(authority_key_id, ''))
WHERE revoked_at IS NULL;

-- +goose Down
DROP SCHEMA plugins CASCADE;
