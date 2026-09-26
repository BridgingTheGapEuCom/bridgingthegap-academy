-- +goose Up
CREATE SCHEMA open_badges;

CREATE TABLE open_badges.status_list (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_purpose text NOT NULL CHECK (status_purpose = 'revocation'),
    capacity integer NOT NULL CHECK (capacity >= 131072),
    next_index integer NOT NULL DEFAULT 0 CHECK (next_index >= 0 AND next_index <= capacity),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX status_list_available_idx ON open_badges.status_list (status_purpose, next_index, id);

CREATE TABLE open_badges.status_list_entry (
    certificate_id uuid PRIMARY KEY REFERENCES credentials.certificate(id) ON DELETE RESTRICT,
    status_list_id uuid NOT NULL REFERENCES open_badges.status_list(id) ON DELETE RESTRICT,
    status_list_index integer NOT NULL CHECK (status_list_index >= 0),
    status_purpose text NOT NULL CHECK (status_purpose = 'revocation'),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT status_list_entry_index_unique UNIQUE (status_list_id, status_list_index)
);
-- +goose Down
DROP SCHEMA open_badges CASCADE;
