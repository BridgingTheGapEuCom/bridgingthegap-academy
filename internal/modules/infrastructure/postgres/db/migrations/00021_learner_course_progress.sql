-- +goose Up
CREATE SCHEMA progress;

CREATE TABLE progress.course_progress (
    learner_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    course_version_id uuid NOT NULL REFERENCES courses.course_version(id) ON DELETE RESTRICT,
    revision bigint NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (learner_user_id, course_version_id)
);

CREATE TABLE progress.lesson_completion (
    learner_user_id uuid NOT NULL,
    course_version_id uuid NOT NULL,
    lesson_key text NOT NULL,
    completed_at timestamptz NOT NULL,
    PRIMARY KEY (learner_user_id, course_version_id, lesson_key),
    FOREIGN KEY (learner_user_id, course_version_id) REFERENCES progress.course_progress(learner_user_id, course_version_id) ON DELETE RESTRICT,
    FOREIGN KEY (course_version_id, lesson_key) REFERENCES courses.lesson(course_version_id, stable_key) ON DELETE RESTRICT
);

-- +goose Down
DROP SCHEMA progress CASCADE;
