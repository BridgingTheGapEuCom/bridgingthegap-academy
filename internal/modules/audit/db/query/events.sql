-- name: AppendEvent :one
INSERT INTO audit.events (action, actor_kind, actor_user_id, resource_type, resource_id, authentication_method, outcome, operation_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: ListEventsForResource :many
SELECT * FROM audit.events
WHERE resource_type = $1 AND resource_id = $2
ORDER BY occurred_at DESC;

-- name: GetEventByOperationID :one
SELECT * FROM audit.events WHERE operation_id = $1;
