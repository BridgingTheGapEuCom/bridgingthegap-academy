-- name: CreateAssessment :one
INSERT INTO assessments.assessment (owner_draft_id, title, definition, created_by_user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAssessment :one
SELECT *
FROM assessments.assessment
WHERE id = $1;

-- name: UpdateAssessment :one
UPDATE assessments.assessment
SET title = $3,
    definition = $4,
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $2
RETURNING *;
