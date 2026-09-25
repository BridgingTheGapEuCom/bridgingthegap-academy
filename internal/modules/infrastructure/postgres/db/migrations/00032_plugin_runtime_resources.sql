-- +goose Up
CREATE TABLE plugins.installed_resource (
    installation_id uuid NOT NULL REFERENCES plugins.installed_release(installation_id) ON DELETE CASCADE,
    resource_path text NOT NULL,
    sha256_digest text NOT NULL CHECK (sha256_digest ~ '^[0-9a-f]{64}$'),
    byte_size bigint NOT NULL CHECK (byte_size >= 0),
    content bytea NOT NULL,
    PRIMARY KEY (installation_id, resource_path),
    CHECK (octet_length(content) = byte_size)
);

-- +goose Down
DROP TABLE plugins.installed_resource;
