-- +goose Up
CREATE SCHEMA audit;

CREATE TABLE audit.events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    action text NOT NULL CHECK (action IN ('ADMINISTRATOR_BOOTSTRAPPED')),
    actor_kind text NOT NULL CHECK (actor_kind IN ('SYSTEM')),
    resource_type text NOT NULL CHECK (resource_type IN ('IDENTITY_USER')),
    resource_id uuid NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('SUCCESS')),
    operation_id uuid NOT NULL UNIQUE,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_resource_idx ON audit.events (resource_type, resource_id, occurred_at DESC);
