-- +goose Up
CREATE SCHEMA translations;

CREATE TABLE translations.course_translation (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_course_id uuid NOT NULL,
    source_course_version_id uuid NOT NULL,
    source_version text NOT NULL CHECK (source_version ~ '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$'),
    source_language text NOT NULL CHECK (char_length(btrim(source_language)) BETWEEN 2 AND 64),
    target_language text NOT NULL CHECK (char_length(btrim(target_language)) BETWEEN 2 AND 64),
    creator_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'PUBLISHED')),
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    translated_tree jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT translation_source_version_context FOREIGN KEY (source_course_version_id, source_course_id)
        REFERENCES courses.course_version(id, course_id) ON DELETE RESTRICT,
    CONSTRAINT translation_source_target_language_distinct CHECK (source_language <> target_language),
    CONSTRAINT translation_source_version_language_unique UNIQUE (source_course_version_id, target_language),
    CONSTRAINT translation_source_binding_unique UNIQUE (id, source_course_id, source_course_version_id, source_version, source_language, target_language)
);

CREATE TABLE translations.translation_publication (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    translation_id uuid NOT NULL,
    source_course_id uuid NOT NULL,
    source_course_version_id uuid NOT NULL,
    source_version text NOT NULL CHECK (source_version ~ '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$'),
    source_language text NOT NULL CHECK (char_length(btrim(source_language)) BETWEEN 2 AND 64),
    target_language text NOT NULL CHECK (char_length(btrim(target_language)) BETWEEN 2 AND 64),
    translation_revision bigint NOT NULL CHECK (translation_revision > 0),
    translated_tree jsonb NOT NULL,
    published_at timestamptz NOT NULL,
    CONSTRAINT translation_publication_source_context FOREIGN KEY (source_course_version_id, source_course_id)
        REFERENCES courses.course_version(id, course_id) ON DELETE RESTRICT,
    CONSTRAINT translation_publication_language_distinct CHECK (source_language <> target_language),
    CONSTRAINT translation_publication_revision_unique UNIQUE (translation_id, translation_revision),
    CONSTRAINT translation_publication_translation_binding FOREIGN KEY (translation_id, source_course_id, source_course_version_id, source_version, source_language, target_language)
        REFERENCES translations.course_translation(id, source_course_id, source_course_version_id, source_version, source_language, target_language) ON DELETE RESTRICT
);
CREATE INDEX translation_publication_latest_idx ON translations.translation_publication(source_course_version_id, target_language, translation_revision DESC, id DESC);

-- +goose Down
DROP SCHEMA translations CASCADE;
