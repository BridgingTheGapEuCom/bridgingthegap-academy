-- +goose Up
CREATE TABLE assessments.assessment_attempt (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    learner_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    course_version_id uuid NOT NULL REFERENCES courses.course_version(id) ON DELETE RESTRICT,
    assessment_key uuid NOT NULL,
    state text NOT NULL DEFAULT 'IN_PROGRESS',
    revision bigint NOT NULL DEFAULT 1,
    responses jsonb NOT NULL DEFAULT '{"responses":[]}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    submitted_at timestamptz,
    CONSTRAINT assessment_attempt_binding FOREIGN KEY (course_version_id, assessment_key)
        REFERENCES courses.course_version_assessment_binding(course_version_id, assessment_key) ON DELETE RESTRICT,
    CONSTRAINT assessment_attempt_state CHECK (state IN ('IN_PROGRESS', 'SUBMITTED')),
    CONSTRAINT assessment_attempt_revision CHECK (revision >= 1),
    CONSTRAINT assessment_attempt_responses_shape CHECK (
        jsonb_typeof(responses) = 'object'
        AND jsonb_typeof(responses->'responses') = 'array'
    ),
    CONSTRAINT assessment_attempt_submission_state CHECK (
        (state = 'IN_PROGRESS' AND submitted_at IS NULL)
        OR (state = 'SUBMITTED' AND submitted_at IS NOT NULL)
    )
);

CREATE INDEX assessment_attempt_learner_context_idx
    ON assessments.assessment_attempt (learner_user_id, course_version_id, assessment_key, created_at DESC, id DESC);

-- +goose Down
DROP TABLE assessments.assessment_attempt;
