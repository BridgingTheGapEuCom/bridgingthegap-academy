-- +goose Up
ALTER TABLE audit.events DROP CONSTRAINT events_action_check;
ALTER TABLE audit.events ADD CONSTRAINT events_action_check
    CHECK (action IN ('ADMINISTRATOR_BOOTSTRAPPED', 'LOCAL_PASSWORD_LOGIN_COMPLETED', 'SESSION_LOGOUT_COMPLETED'));

ALTER TABLE audit.events DROP CONSTRAINT events_actor_kind_check;
ALTER TABLE audit.events ADD CONSTRAINT events_actor_kind_check CHECK (actor_kind IN ('SYSTEM', 'USER'));

ALTER TABLE audit.events DROP CONSTRAINT events_resource_type_check;
ALTER TABLE audit.events ADD CONSTRAINT events_resource_type_check CHECK (resource_type IN ('IDENTITY_USER', 'IDENTITY_SESSION'));

ALTER TABLE audit.events ADD COLUMN actor_user_id uuid;
ALTER TABLE audit.events ADD COLUMN authentication_method text;
ALTER TABLE audit.events ADD CONSTRAINT events_actor_user_consistent CHECK
    ((actor_kind = 'SYSTEM' AND actor_user_id IS NULL) OR (actor_kind = 'USER' AND actor_user_id IS NOT NULL));
ALTER TABLE audit.events ADD CONSTRAINT events_authentication_method_valid CHECK
    (authentication_method IS NULL OR authentication_method = 'LOCAL_PASSWORD');
ALTER TABLE audit.events ADD CONSTRAINT events_completed_action_shape CHECK
    ((action = 'ADMINISTRATOR_BOOTSTRAPPED' AND actor_kind = 'SYSTEM' AND resource_type = 'IDENTITY_USER' AND authentication_method IS NULL)
     OR (action = 'LOCAL_PASSWORD_LOGIN_COMPLETED' AND actor_kind = 'USER' AND resource_type = 'IDENTITY_SESSION' AND authentication_method IS NOT NULL AND authentication_method = 'LOCAL_PASSWORD')
     OR (action = 'SESSION_LOGOUT_COMPLETED' AND actor_kind = 'USER' AND resource_type = 'IDENTITY_SESSION' AND authentication_method IS NULL));
CREATE INDEX audit_events_actor_user_idx ON audit.events (actor_user_id, occurred_at DESC) WHERE actor_user_id IS NOT NULL;
