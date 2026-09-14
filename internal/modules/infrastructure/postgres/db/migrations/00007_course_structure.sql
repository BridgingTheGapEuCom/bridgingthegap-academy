-- +goose Up
CREATE TABLE courses.module (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_version_id uuid NOT NULL REFERENCES courses.course_version(id) ON DELETE RESTRICT,
    stable_key text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT course_module_key_format CHECK (stable_key ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT course_module_key_length CHECK (char_length(stable_key) BETWEEN 3 AND 96),
    CONSTRAINT course_module_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 240),
    CONSTRAINT course_module_description CHECK (char_length(description) <= 20000),
    CONSTRAINT course_module_position CHECK (position BETWEEN 0 AND 100000),
    CONSTRAINT course_module_version_key_unique UNIQUE (course_version_id, stable_key),
    CONSTRAINT course_module_version_position_unique UNIQUE (course_version_id, position),
    CONSTRAINT course_module_id_version_unique UNIQUE (id, course_version_id)
);

CREATE TABLE courses.lesson (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_version_id uuid NOT NULL,
    module_id uuid NOT NULL,
    stable_key text NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    learning_objectives jsonb NOT NULL,
    estimated_duration_minutes integer,
    position integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT course_lesson_module_version_fk FOREIGN KEY (module_id, course_version_id)
        REFERENCES courses.module(id, course_version_id) ON DELETE RESTRICT,
    CONSTRAINT course_lesson_key_format CHECK (stable_key ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT course_lesson_key_length CHECK (char_length(stable_key) BETWEEN 3 AND 96),
    CONSTRAINT course_lesson_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 240),
    CONSTRAINT course_lesson_description CHECK (char_length(btrim(description)) BETWEEN 1 AND 20000),
    CONSTRAINT course_lesson_objectives CHECK (jsonb_typeof(learning_objectives) = 'array' AND jsonb_array_length(learning_objectives) BETWEEN 1 AND 100),
    CONSTRAINT course_lesson_duration CHECK (estimated_duration_minutes IS NULL OR estimated_duration_minutes BETWEEN 1 AND 1440),
    CONSTRAINT course_lesson_position CHECK (position BETWEEN 0 AND 100000),
    CONSTRAINT course_lesson_version_key_unique UNIQUE (course_version_id, stable_key),
    CONSTRAINT course_lesson_module_position_unique UNIQUE (module_id, position),
    CONSTRAINT course_lesson_id_version_unique UNIQUE (id, course_version_id)
);

CREATE TABLE courses.lesson_prerequisite (
    course_version_id uuid NOT NULL,
    lesson_id uuid NOT NULL,
    prerequisite_lesson_id uuid NOT NULL,
    position integer NOT NULL,
    CONSTRAINT course_lesson_prerequisite_lesson_fk FOREIGN KEY (lesson_id, course_version_id)
        REFERENCES courses.lesson(id, course_version_id) ON DELETE RESTRICT,
    CONSTRAINT course_lesson_prerequisite_target_fk FOREIGN KEY (prerequisite_lesson_id, course_version_id)
        REFERENCES courses.lesson(id, course_version_id) ON DELETE RESTRICT,
    CONSTRAINT course_lesson_prerequisite_not_self CHECK (lesson_id <> prerequisite_lesson_id),
    CONSTRAINT course_lesson_prerequisite_position CHECK (position BETWEEN 0 AND 100000),
    CONSTRAINT course_lesson_prerequisite_unique UNIQUE (lesson_id, prerequisite_lesson_id),
    CONSTRAINT course_lesson_prerequisite_order_unique UNIQUE (lesson_id, position)
);
