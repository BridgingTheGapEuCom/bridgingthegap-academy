-- +goose Up
CREATE TABLE courses.course_version_publication_provenance (
    course_version_id uuid PRIMARY KEY REFERENCES courses.course_version(id) ON DELETE RESTRICT,
    review_id uuid NOT NULL,
    review_revision bigint NOT NULL,
    draft_id uuid NOT NULL,
    draft_revision bigint NOT NULL,
    snapshot_schema_version integer NOT NULL,
    submitted_by_user_id uuid NOT NULL,
    submitted_at timestamptz NOT NULL,
    approved_by_user_id uuid NOT NULL,
    approved_at timestamptz NOT NULL,
    CONSTRAINT course_version_publication_review_unique UNIQUE (review_id),
    CONSTRAINT course_version_publication_generated_id_distinct CHECK (course_version_id <> review_id AND course_version_id <> draft_id),
    CONSTRAINT course_version_publication_review_revision CHECK (review_revision >= 1),
    CONSTRAINT course_version_publication_draft_revision CHECK (draft_revision >= 1),
    CONSTRAINT course_version_publication_snapshot_version CHECK (snapshot_schema_version >= 1)
);

ALTER TABLE courses.module
    ADD COLUMN source_module_id uuid,
    ADD CONSTRAINT course_module_generated_id_distinct CHECK (source_module_id IS NULL OR id <> source_module_id);

CREATE UNIQUE INDEX course_module_version_source_unique
    ON courses.module (course_version_id, source_module_id)
    WHERE source_module_id IS NOT NULL;

ALTER TABLE courses.lesson
    ADD COLUMN source_lesson_id uuid,
    ADD CONSTRAINT course_lesson_generated_id_distinct CHECK (source_lesson_id IS NULL OR id <> source_lesson_id);

CREATE UNIQUE INDEX course_lesson_version_source_unique
    ON courses.lesson (course_version_id, source_lesson_id)
    WHERE source_lesson_id IS NOT NULL;
