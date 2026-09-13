-- name: AppendEvent :one
INSERT INTO audit.events (action, actor_kind, resource_type, resource_id, outcome, operation_id)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: ListEventsForResource :many
SELECT * FROM audit.events
WHERE resource_type = $1 AND resource_id = $2
ORDER BY occurred_at DESC;
