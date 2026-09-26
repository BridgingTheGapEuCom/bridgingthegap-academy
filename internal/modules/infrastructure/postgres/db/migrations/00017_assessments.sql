-- +goose Up
CREATE SCHEMA assessments;

CREATE TABLE assessments.assessment (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_draft_id uuid NOT NULL,
    title text NOT NULL,
    revision bigint NOT NULL DEFAULT 1,
    definition jsonb NOT NULL,
    created_by_user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT assessment_title CHECK (
        title = btrim(title) AND char_length(title) BETWEEN 1 AND 240
    ),
    CONSTRAINT assessment_revision CHECK (revision >= 1),
    CONSTRAINT assessment_definition_shape CHECK (
        jsonb_typeof(definition) = 'object'
        AND jsonb_typeof(definition->'questions') = 'array'
    )
);

CREATE INDEX assessment_owner_draft_idx
    ON assessments.assessment (owner_draft_id, updated_at DESC, id DESC);
