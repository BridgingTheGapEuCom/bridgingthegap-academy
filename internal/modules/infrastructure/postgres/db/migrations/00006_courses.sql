-- +goose Up
CREATE SCHEMA courses;

CREATE TABLE courses.course (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT course_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT course_slug_length CHECK (char_length(slug) BETWEEN 3 AND 96)
);

CREATE TABLE courses.course_version (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id uuid NOT NULL REFERENCES courses.course(id) ON DELETE RESTRICT,
    version text NOT NULL,
    version_major integer GENERATED ALWAYS AS (split_part(version, '.', 1)::integer) STORED,
    version_minor integer GENERATED ALWAYS AS (split_part(version, '.', 2)::integer) STORED,
    version_patch integer GENERATED ALWAYS AS (split_part(version, '.', 3)::integer) STORED,
    status text NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    learning_objectives jsonb NOT NULL,
    source_language text NOT NULL,
    changelog text NOT NULL,
    license_kind text NOT NULL,
    license_identifier text,
    license_display_name text NOT NULL,
    license_url text,
    license_custom_text text,
    attribution jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz NOT NULL,
    CONSTRAINT course_version_unique UNIQUE (course_id, version),
    CONSTRAINT course_version_format CHECK (version ~ '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$'),
    CONSTRAINT course_version_status CHECK (status IN ('PUBLISHED', 'DEPRECATED', 'ARCHIVED', 'WITHDRAWN')),
    CONSTRAINT course_version_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 240),
    CONSTRAINT course_version_description CHECK (char_length(btrim(description)) BETWEEN 1 AND 20000),
    CONSTRAINT course_version_objectives CHECK (jsonb_typeof(learning_objectives) = 'array' AND jsonb_array_length(learning_objectives) BETWEEN 1 AND 100),
    CONSTRAINT course_version_language CHECK (source_language ~ '^[a-z]{2,3}(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?(-[a-z0-9]{2,8})*$' AND char_length(source_language) <= 64),
    CONSTRAINT course_version_changelog CHECK (char_length(btrim(changelog)) BETWEEN 1 AND 20000),
    CONSTRAINT course_version_license_kind CHECK (license_kind IN ('STANDARD', 'ALL_RIGHTS_RESERVED', 'CUSTOM')),
    CONSTRAINT course_version_license_display_name CHECK (char_length(btrim(license_display_name)) BETWEEN 1 AND 256),
    CONSTRAINT course_version_license_url CHECK (license_url IS NULL OR (char_length(license_url) <= 2048 AND license_url ~ '^https?://[^[:space:]]+$')),
    CONSTRAINT course_version_license_shape CHECK (
        (license_kind = 'STANDARD' AND license_identifier IS NOT NULL AND char_length(btrim(license_identifier)) BETWEEN 1 AND 128 AND license_custom_text IS NULL)
        OR (license_kind = 'ALL_RIGHTS_RESERVED' AND license_identifier IS NULL AND license_custom_text IS NULL)
        OR (license_kind = 'CUSTOM' AND license_identifier IS NULL AND license_custom_text IS NOT NULL AND char_length(btrim(license_custom_text)) BETWEEN 1 AND 20000)
    ),
    CONSTRAINT course_version_attribution CHECK (jsonb_typeof(attribution) = 'array' AND jsonb_array_length(attribution) BETWEEN 1 AND 100)
);

CREATE INDEX course_version_course_order_idx ON courses.course_version (course_id, version_major DESC, version_minor DESC, version_patch DESC, id DESC);
CREATE INDEX course_version_status_idx ON courses.course_version (status);
