-- +goose Up
CREATE TABLE authoring.review_cycle (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL REFERENCES authoring.course_draft(id),
    draft_revision bigint NOT NULL,
    snapshot_schema_version integer NOT NULL DEFAULT 1,
    snapshot jsonb NOT NULL,
    status text NOT NULL DEFAULT 'IN_REVIEW',
    revision bigint NOT NULL DEFAULT 1,
    submitted_by_user_id uuid NOT NULL,
    submitted_at timestamptz NOT NULL DEFAULT now(),
    decided_by_user_id uuid,
    decided_at timestamptz,
    CONSTRAINT authoring_review_frozen_revision CHECK (draft_revision >= 1),
    CONSTRAINT authoring_review_snapshot_schema CHECK (snapshot_schema_version = 1),
    CONSTRAINT authoring_review_snapshot_shape CHECK ((
        jsonb_typeof(snapshot) = 'object'
        AND snapshot ->> 'schemaVersion' = snapshot_schema_version::text
        AND jsonb_typeof(snapshot -> 'draft') = 'object'
        AND snapshot -> 'draft' ->> 'id' = draft_id::text
        AND snapshot -> 'draft' ->> 'revision' = draft_revision::text
        AND jsonb_typeof(snapshot -> 'modules') = 'array'
    ) IS TRUE),
    CONSTRAINT authoring_review_status CHECK (status IN ('IN_REVIEW', 'APPROVED', 'CHANGES_REQUESTED')),
    CONSTRAINT authoring_review_revision CHECK (revision >= 1),
    CONSTRAINT authoring_review_decision_fields CHECK (
        (status = 'IN_REVIEW' AND decided_by_user_id IS NULL AND decided_at IS NULL)
        OR (status IN ('APPROVED', 'CHANGES_REQUESTED') AND decided_by_user_id IS NOT NULL AND decided_at IS NOT NULL)
    ),
    CONSTRAINT authoring_review_draft_revision_unique UNIQUE (draft_id, draft_revision),
    CONSTRAINT authoring_review_id_draft_unique UNIQUE (id, draft_id)
);
CREATE UNIQUE INDEX authoring_review_one_active ON authoring.review_cycle (draft_id) WHERE status = 'IN_REVIEW';
CREATE INDEX authoring_review_history ON authoring.review_cycle (draft_id, submitted_at DESC, id DESC);
CREATE INDEX authoring_review_approved_revision ON authoring.review_cycle (draft_id, draft_revision) WHERE status = 'APPROVED';

CREATE TABLE authoring.review_event (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id uuid NOT NULL REFERENCES authoring.review_cycle(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    actor_user_id uuid NOT NULL,
    message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT authoring_review_event_type CHECK (event_type IN ('SUBMITTED', 'APPROVED', 'CHANGES_REQUESTED')),
    CONSTRAINT authoring_review_event_message CHECK (message IS NULL OR char_length(message) <= 20000)
);
CREATE INDEX authoring_review_event_history ON authoring.review_event (review_id, created_at, id);
CREATE UNIQUE INDEX authoring_review_one_submission ON authoring.review_event (review_id)
WHERE event_type = 'SUBMITTED';
CREATE UNIQUE INDEX authoring_review_one_decision ON authoring.review_event (review_id)
WHERE event_type IN ('APPROVED', 'CHANGES_REQUESTED');

-- Frozen provenance and append-only decisions must not be rewritten.
-- +goose StatementBegin
CREATE FUNCTION authoring.protect_review_provenance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.draft_id <> OLD.draft_id
       OR NEW.draft_revision <> OLD.draft_revision
       OR NEW.snapshot_schema_version <> OLD.snapshot_schema_version
       OR NEW.snapshot <> OLD.snapshot
       OR NEW.submitted_by_user_id <> OLD.submitted_by_user_id
       OR NEW.submitted_at <> OLD.submitted_at THEN
        RAISE EXCEPTION 'review provenance is immutable' USING ERRCODE = '23514';
    END IF;
    IF OLD.status <> 'IN_REVIEW' OR NEW.status NOT IN ('APPROVED', 'CHANGES_REQUESTED') THEN
        RAISE EXCEPTION 'invalid review transition' USING ERRCODE = '23514';
    END IF;
    IF NEW.revision <> OLD.revision + 1 THEN
        RAISE EXCEPTION 'invalid review revision' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER authoring_review_provenance_immutable
BEFORE UPDATE ON authoring.review_cycle FOR EACH ROW EXECUTE FUNCTION authoring.protect_review_provenance();

-- +goose StatementBegin
CREATE FUNCTION authoring.protect_review_event_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'review events are append-only' USING ERRCODE = '23514';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER authoring_review_event_no_update
BEFORE UPDATE OR DELETE ON authoring.review_event FOR EACH ROW EXECUTE FUNCTION authoring.protect_review_event_history();
