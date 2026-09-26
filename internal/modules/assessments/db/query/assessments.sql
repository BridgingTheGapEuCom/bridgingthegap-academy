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

-- name: ListAssessmentSummariesForDraft :many
SELECT id, title, jsonb_array_length(definition->'questions') AS question_count, revision, updated_at
FROM assessments.assessment
WHERE owner_draft_id = $1
ORDER BY updated_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountAssessmentSummariesForDraft :one
SELECT count(*)
FROM assessments.assessment
WHERE owner_draft_id = $1;
