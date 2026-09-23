-- +goose Up
CREATE SCHEMA credentials;

ALTER TABLE courses.course_version
    ADD CONSTRAINT course_version_id_course_unique UNIQUE (id, course_id);

CREATE TABLE credentials.certificate (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    learner_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    course_id uuid NOT NULL,
    course_version_id uuid NOT NULL,
    course_title text NOT NULL CHECK (char_length(btrim(course_title)) BETWEEN 1 AND 240),
    course_version text NOT NULL CHECK (course_version ~ '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$'),
    language text NOT NULL CHECK (char_length(btrim(language)) BETWEEN 2 AND 64),
    criteria text NOT NULL CHECK (char_length(btrim(criteria)) BETWEEN 1 AND 4000),
    issuer_id text NOT NULL CHECK (char_length(btrim(issuer_id)) BETWEEN 1 AND 256),
    issuer_name text NOT NULL CHECK (char_length(btrim(issuer_name)) BETWEEN 1 AND 256),
    issued_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'REVOKED')),
    revoked_at timestamptz,
    CONSTRAINT certificate_course_version_context FOREIGN KEY (course_version_id, course_id)
        REFERENCES courses.course_version(id, course_id) ON DELETE RESTRICT,
    CONSTRAINT certificate_issuance_unique UNIQUE (learner_user_id, course_version_id),
    CONSTRAINT certificate_revocation_state CHECK (
        (status = 'ACTIVE' AND revoked_at IS NULL)
        OR (status = 'REVOKED' AND revoked_at IS NOT NULL AND revoked_at >= issued_at)
    )
);

CREATE INDEX certificate_learner_issued_order_idx ON credentials.certificate (learner_user_id, issued_at DESC, id DESC);

-- +goose Down
DROP SCHEMA credentials CASCADE;
ALTER TABLE courses.course_version
    DROP CONSTRAINT course_version_id_course_unique;
