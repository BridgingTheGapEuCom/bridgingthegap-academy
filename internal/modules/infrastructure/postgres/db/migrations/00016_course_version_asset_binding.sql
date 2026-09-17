-- +goose Up
CREATE TABLE courses.course_version_asset_binding (
    course_version_id uuid NOT NULL REFERENCES courses.course_version(id) ON DELETE RESTRICT,
    asset_key uuid NOT NULL,
    storage_object_id uuid NOT NULL,
    original_filename text NOT NULL,
    media_type text NOT NULL,
    byte_size bigint NOT NULL,
    sha256_digest text NOT NULL,
    PRIMARY KEY (course_version_id, asset_key),
    CONSTRAINT course_version_asset_filename CHECK (
        original_filename = btrim(original_filename)
        AND char_length(original_filename) BETWEEN 1 AND 255
        AND original_filename NOT IN ('.', '..')
        AND position('/' IN original_filename) = 0
        AND position(chr(92) IN original_filename) = 0
        AND original_filename !~ '[[:cntrl:]]'
    ),
    CONSTRAINT course_version_asset_media_type CHECK (
        char_length(media_type) BETWEEN 3 AND 127
        AND media_type ~ '^[a-z0-9][a-z0-9!#$&^_.+-]*/[a-z0-9][a-z0-9!#$&^_.+-]*$'
    ),
    CONSTRAINT course_version_asset_size CHECK (byte_size > 0),
    CONSTRAINT course_version_asset_digest CHECK (sha256_digest ~ '^[0-9a-f]{64}$')
);

CREATE INDEX course_version_asset_storage_idx
    ON courses.course_version_asset_binding (storage_object_id);
