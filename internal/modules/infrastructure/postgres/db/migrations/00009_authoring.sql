-- +goose Up
CREATE SCHEMA authoring;

-- A draft targets a stable Courses identity but owns all mutable content itself.
CREATE TABLE authoring.course_draft (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id uuid NOT NULL REFERENCES courses.course(id) ON DELETE RESTRICT,
    intended_version text NOT NULL,
    source_language text NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    learning_objectives jsonb NOT NULL,
    changelog text NOT NULL,
    license jsonb NOT NULL,
    status text NOT NULL DEFAULT 'ACTIVE',
    revision bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT authoring_draft_version CHECK (intended_version ~ '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$'),
    CONSTRAINT authoring_draft_language CHECK (source_language ~ '^[a-z]{2,3}(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?(-[a-z0-9]{2,8})*$' AND char_length(source_language) <= 64),
    CONSTRAINT authoring_draft_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 240),
    CONSTRAINT authoring_draft_description CHECK (char_length(btrim(description)) BETWEEN 1 AND 20000),
    CONSTRAINT authoring_draft_objectives CHECK (jsonb_typeof(learning_objectives) = 'array' AND jsonb_array_length(learning_objectives) BETWEEN 1 AND 100),
    CONSTRAINT authoring_draft_changelog CHECK (char_length(btrim(changelog)) BETWEEN 1 AND 20000),
    CONSTRAINT authoring_draft_license CHECK (jsonb_typeof(license) = 'object'),
    CONSTRAINT authoring_draft_status CHECK (status IN ('ACTIVE', 'ABANDONED')),
    CONSTRAINT authoring_draft_revision CHECK (revision >= 1),
    CONSTRAINT authoring_draft_timestamps CHECK (updated_at >= created_at)
);

CREATE TABLE authoring.workspace (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL UNIQUE REFERENCES authoring.course_draft(id) ON DELETE CASCADE,
    created_by_user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_activity_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT authoring_workspace_timestamps CHECK (last_activity_at >= created_at)
);

CREATE TABLE authoring.workspace_member (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES authoring.workspace(id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    role text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    CONSTRAINT authoring_member_role CHECK (role IN ('MAINTAINER', 'AUTHOR')),
    CONSTRAINT authoring_member_revoked_time CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE UNIQUE INDEX authoring_member_active_unique ON authoring.workspace_member (workspace_id, user_id) WHERE revoked_at IS NULL;
CREATE INDEX authoring_member_workspace_idx ON authoring.workspace_member (workspace_id, created_at, id);

CREATE TABLE authoring.module (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL REFERENCES authoring.course_draft(id) ON DELETE CASCADE,
    stable_key text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    position integer NOT NULL,
    revision bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT authoring_module_key CHECK (stable_key ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(stable_key) BETWEEN 3 AND 96),
    CONSTRAINT authoring_module_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 240),
    CONSTRAINT authoring_module_description CHECK (char_length(description) <= 20000),
    CONSTRAINT authoring_module_position CHECK (position BETWEEN 0 AND 100000),
    CONSTRAINT authoring_module_revision CHECK (revision >= 1),
    CONSTRAINT authoring_module_timestamps CHECK (updated_at >= created_at),
    CONSTRAINT authoring_module_draft_key_unique UNIQUE (draft_id, stable_key),
    CONSTRAINT authoring_module_draft_position_unique UNIQUE (draft_id, position) DEFERRABLE INITIALLY IMMEDIATE,
    CONSTRAINT authoring_module_id_draft_unique UNIQUE (id, draft_id)
);

CREATE TABLE authoring.lesson (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL,
    module_id uuid NOT NULL,
    stable_key text NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    learning_objectives jsonb NOT NULL,
    estimated_duration_minutes integer,
    position integer NOT NULL,
    content jsonb NOT NULL,
    revision bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT authoring_lesson_module_fk FOREIGN KEY (module_id, draft_id) REFERENCES authoring.module(id, draft_id) ON DELETE CASCADE,
    CONSTRAINT authoring_lesson_key CHECK (stable_key ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(stable_key) BETWEEN 3 AND 96),
    CONSTRAINT authoring_lesson_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 240),
    CONSTRAINT authoring_lesson_description CHECK (char_length(btrim(description)) BETWEEN 1 AND 20000),
    CONSTRAINT authoring_lesson_objectives CHECK (jsonb_typeof(learning_objectives) = 'array' AND jsonb_array_length(learning_objectives) BETWEEN 1 AND 100),
    CONSTRAINT authoring_lesson_duration CHECK (estimated_duration_minutes IS NULL OR estimated_duration_minutes BETWEEN 1 AND 1440),
    CONSTRAINT authoring_lesson_position CHECK (position BETWEEN 0 AND 100000),
    CONSTRAINT authoring_lesson_content CHECK (jsonb_typeof(content) = 'object' AND content ->> 'schemaVersion' = '1' AND jsonb_typeof(content -> 'blocks') = 'array'),
    CONSTRAINT authoring_lesson_revision CHECK (revision >= 1),
    CONSTRAINT authoring_lesson_timestamps CHECK (updated_at >= created_at),
    CONSTRAINT authoring_lesson_draft_key_unique UNIQUE (draft_id, stable_key),
    CONSTRAINT authoring_lesson_module_position_unique UNIQUE (module_id, position) DEFERRABLE INITIALLY IMMEDIATE,
    CONSTRAINT authoring_lesson_id_draft_unique UNIQUE (id, draft_id)
);
CREATE INDEX authoring_lesson_module_order_idx ON authoring.lesson (module_id, position, id);

-- Both ends carry draft_id so PostgreSQL enforces same-draft advisory references.
CREATE TABLE authoring.lesson_prerequisite (
    draft_id uuid NOT NULL,
    lesson_id uuid NOT NULL,
    prerequisite_lesson_id uuid NOT NULL,
    position integer NOT NULL,
    CONSTRAINT authoring_prerequisite_source_fk FOREIGN KEY (lesson_id, draft_id) REFERENCES authoring.lesson(id, draft_id) ON DELETE CASCADE,
    CONSTRAINT authoring_prerequisite_target_fk FOREIGN KEY (prerequisite_lesson_id, draft_id) REFERENCES authoring.lesson(id, draft_id) ON DELETE CASCADE,
    CONSTRAINT authoring_prerequisite_not_self CHECK (lesson_id <> prerequisite_lesson_id),
    CONSTRAINT authoring_prerequisite_position CHECK (position BETWEEN 0 AND 100000),
    CONSTRAINT authoring_prerequisite_unique UNIQUE (lesson_id, prerequisite_lesson_id),
    CONSTRAINT authoring_prerequisite_order_unique UNIQUE (lesson_id, position)
);
CREATE INDEX authoring_prerequisite_target_idx ON authoring.lesson_prerequisite (prerequisite_lesson_id);
