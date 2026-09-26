-- name: CreateAssessmentAttempt :one
INSERT INTO assessments.assessment_attempt (learner_user_id, course_version_id, assessment_key, responses)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAssessmentAttempt :one
SELECT *
FROM assessments.assessment_attempt
WHERE id = $1;

-- name: UpdateAssessmentAttempt :one
UPDATE assessments.assessment_attempt
SET responses = $3,
    revision = revision + 1,
    updated_at = now()
WHERE id = $1 AND revision = $2 AND state = 'IN_PROGRESS'
RETURNING *;

-- name: SubmitAssessmentAttempt :one
UPDATE assessments.assessment_attempt
SET state = 'SUBMITTED',
    submitted_at = $3,
    correct_count = $4,
    total_count = $5,
    revision = revision + 1,
    updated_at = $3
WHERE id = $1 AND revision = $2 AND state = 'IN_PROGRESS'
RETURNING *;
