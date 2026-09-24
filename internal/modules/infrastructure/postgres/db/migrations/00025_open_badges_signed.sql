-- +goose Up
ALTER TABLE open_badges.status_list ADD COLUMN lifecycle_revision bigint NOT NULL DEFAULT 0;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION open_badges.bump_status_list_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        UPDATE open_badges.status_list AS list SET lifecycle_revision = lifecycle_revision + 1
        FROM open_badges.status_list_entry AS entry
        WHERE entry.certificate_id = NEW.id AND list.id = entry.status_list_id;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER certificate_status_list_revision AFTER UPDATE OF status ON credentials.certificate
FOR EACH ROW EXECUTE FUNCTION open_badges.bump_status_list_revision();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION open_badges.bump_status_list_allocation_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE open_badges.status_list SET lifecycle_revision = lifecycle_revision + 1 WHERE id = NEW.status_list_id;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER status_list_allocation_revision AFTER INSERT ON open_badges.status_list_entry
FOR EACH ROW EXECUTE FUNCTION open_badges.bump_status_list_allocation_revision();

CREATE TABLE open_badges.verification_method (
    id text PRIMARY KEY,
    controller text NOT NULL,
    public_key_multibase text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX verification_method_controller_idx ON open_badges.verification_method (controller, id);

CREATE TABLE open_badges.signed_credential (
    certificate_id uuid PRIMARY KEY REFERENCES credentials.certificate(id) ON DELETE RESTRICT,
    key_id text NOT NULL REFERENCES open_badges.verification_method(id) ON DELETE RESTRICT,
    secured_document bytea NOT NULL CHECK (octet_length(secured_document) > 0),
    created_at timestamptz NOT NULL
);

CREATE TABLE open_badges.published_status_list (
    status_list_id uuid PRIMARY KEY REFERENCES open_badges.status_list(id) ON DELETE RESTRICT,
    key_id text NOT NULL REFERENCES open_badges.verification_method(id) ON DELETE RESTRICT,
    secured_document bytea NOT NULL CHECK (octet_length(secured_document) > 0),
    encoded_list text NOT NULL,
    generated_at timestamptz NOT NULL,
    lifecycle_revision bigint NOT NULL
);
-- +goose Down
DROP TABLE open_badges.published_status_list;
DROP TABLE open_badges.signed_credential;
DROP TABLE open_badges.verification_method;
DROP TRIGGER status_list_allocation_revision ON open_badges.status_list_entry;
DROP FUNCTION open_badges.bump_status_list_allocation_revision();
DROP TRIGGER certificate_status_list_revision ON credentials.certificate;
DROP FUNCTION open_badges.bump_status_list_revision();
ALTER TABLE open_badges.status_list DROP COLUMN lifecycle_revision;
