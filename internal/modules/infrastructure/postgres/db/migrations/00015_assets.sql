-- +goose Up
CREATE SCHEMA assets;

CREATE TABLE assets.asset (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_draft_id uuid NOT NULL,
    original_filename text NOT NULL,
    media_type text NOT NULL,
    byte_size bigint,
    sha256_digest text,
    storage_object_id uuid UNIQUE,
    lifecycle text NOT NULL,
    created_by_user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_original_filename CHECK (
        original_filename = btrim(original_filename)
        AND char_length(original_filename) BETWEEN 1 AND 255
        AND original_filename NOT IN ('.', '..')
        AND position('/' IN original_filename) = 0
        AND position(chr(92) IN original_filename) = 0
        AND original_filename !~ '[[:cntrl:]]'
    ),
    CONSTRAINT asset_media_type CHECK (
        char_length(media_type) BETWEEN 3 AND 127
        AND media_type ~ '^[a-z0-9][a-z0-9!#$&^_.+-]*/[a-z0-9][a-z0-9!#$&^_.+-]*$'
    ),
    CONSTRAINT asset_lifecycle CHECK (lifecycle IN ('PENDING', 'AVAILABLE')),
    CONSTRAINT asset_content_shape CHECK (
        (lifecycle = 'PENDING' AND byte_size IS NULL AND sha256_digest IS NULL AND storage_object_id IS NULL)
        OR
        (lifecycle = 'AVAILABLE' AND byte_size > 0 AND sha256_digest ~ '^[0-9a-f]{64}$' AND storage_object_id IS NOT NULL)
    )
);

CREATE INDEX asset_owner_draft_idx ON assets.asset (owner_draft_id, created_at, id);
CREATE INDEX asset_lifecycle_idx ON assets.asset (lifecycle, created_at, id);
