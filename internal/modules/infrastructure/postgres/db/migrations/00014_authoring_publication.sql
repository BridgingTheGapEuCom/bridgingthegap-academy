-- +goose Up
ALTER TABLE authoring.review_cycle
    ADD CONSTRAINT review_cycle_id_draft_unique UNIQUE (id, draft_id);

CREATE TABLE authoring.review_publication (
    review_id uuid PRIMARY KEY,
    review_revision bigint NOT NULL,
    draft_id uuid NOT NULL REFERENCES authoring.course_draft(id) ON DELETE RESTRICT,
    draft_revision bigint NOT NULL,
    course_id uuid NOT NULL,
    course_version text NOT NULL,
    course_version_id uuid NOT NULL UNIQUE,
    published_at timestamptz NOT NULL,
    published_by_user_id uuid NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT review_publication_review_draft_fk FOREIGN KEY (review_id, draft_id)
        REFERENCES authoring.review_cycle(id, draft_id) ON DELETE RESTRICT,
    CONSTRAINT review_publication_review_revision CHECK (review_revision >= 1),
    CONSTRAINT review_publication_draft_revision CHECK (draft_revision >= 1),
    CONSTRAINT review_publication_version CHECK (course_version ~ '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$'),
    CONSTRAINT review_publication_course_version_unique UNIQUE (course_id, course_version)
);
