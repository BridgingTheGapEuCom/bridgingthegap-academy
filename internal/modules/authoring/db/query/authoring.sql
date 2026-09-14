-- name: CreateDraft :one
INSERT INTO authoring.course_draft (course_id, intended_version, source_language, title, description, learning_objectives, changelog, license)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetDraft :one
SELECT * FROM authoring.course_draft WHERE id = $1;

-- name: UpdateDraftMetadata :one
UPDATE authoring.course_draft
SET intended_version = $3, source_language = $4, title = $5, description = $6,
    learning_objectives = $7, changelog = $8, license = $9,
    revision = revision + 1, updated_at = now()
WHERE id = $1 AND revision = $2 AND status = 'ACTIVE'
RETURNING *;

-- name: AbandonDraft :one
UPDATE authoring.course_draft
SET status = 'ABANDONED', revision = revision + 1, updated_at = now()
WHERE id = $1 AND revision = $2 AND status = 'ACTIVE'
RETURNING *;

-- name: BumpDraftRevision :one
UPDATE authoring.course_draft
SET revision = revision + 1, updated_at = now()
WHERE id = $1 AND revision = $2 AND status = 'ACTIVE'
RETURNING *;

-- name: TouchDraft :exec
UPDATE authoring.course_draft SET revision = revision + 1, updated_at = now()
WHERE id = $1 AND status = 'ACTIVE';

-- name: CreateWorkspace :one
INSERT INTO authoring.workspace (draft_id, created_by_user_id)
VALUES ($1, $2) RETURNING *;

-- name: GetWorkspace :one
SELECT * FROM authoring.workspace WHERE draft_id = $1;

-- name: TouchWorkspace :exec
UPDATE authoring.workspace SET last_activity_at = now() WHERE draft_id = $1;

-- name: AddMember :one
INSERT INTO authoring.workspace_member (workspace_id, user_id, role)
SELECT $1, $2, $3 FROM authoring.workspace AS w
JOIN authoring.course_draft AS d ON d.id = w.draft_id
WHERE w.id = $1 AND d.status = 'ACTIVE'
RETURNING *;

-- name: RevokeMember :one
UPDATE authoring.workspace_member
SET revoked_at = now()
WHERE workspace_id = $1 AND user_id = $2 AND revoked_at IS NULL
RETURNING *;

-- name: ListMembers :many
SELECT * FROM authoring.workspace_member WHERE workspace_id = $1 ORDER BY created_at, id;

-- name: ActiveMembershipForDraft :one
SELECT member.role
FROM authoring.workspace_member AS member
JOIN authoring.workspace AS workspace ON workspace.id = member.workspace_id
WHERE workspace.draft_id = $1 AND member.user_id = $2 AND member.revoked_at IS NULL;

-- name: CreateModule :one
INSERT INTO authoring.module (draft_id, stable_key, title, description, position)
SELECT d.id, $2, $3, $4, $5 FROM authoring.course_draft AS d
WHERE d.id = $1 AND d.status = 'ACTIVE'
RETURNING *;

-- name: GetModule :one
SELECT * FROM authoring.module WHERE id = $1;

-- name: ListModules :many
SELECT * FROM authoring.module WHERE draft_id = $1 ORDER BY position, id;

-- name: UpdateModule :one
UPDATE authoring.module AS m
SET title = $3, description = $4, revision = m.revision + 1, updated_at = now()
WHERE m.id = $1 AND m.revision = $2
  AND EXISTS (SELECT 1 FROM authoring.course_draft AS d WHERE d.id = m.draft_id AND d.status = 'ACTIVE')
RETURNING m.*;

-- name: SetModulePosition :one
UPDATE authoring.module
SET position = $2, revision = revision + 1, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteModule :one
DELETE FROM authoring.module AS m
WHERE m.id = $1 AND m.revision = $2
  AND EXISTS (SELECT 1 FROM authoring.course_draft AS d WHERE d.id = m.draft_id AND d.status = 'ACTIVE')
RETURNING m.id;

-- name: CreateLesson :one
INSERT INTO authoring.lesson (draft_id, module_id, stable_key, title, description, learning_objectives, estimated_duration_minutes, position, content)
SELECT m.draft_id, m.id, $2, $3, $4, $5, $6, $7, $8
FROM authoring.module AS m
JOIN authoring.course_draft AS d ON d.id = m.draft_id
WHERE m.id = $1 AND m.draft_id = $9 AND d.status = 'ACTIVE'
RETURNING *;

-- name: GetLesson :one
SELECT * FROM authoring.lesson WHERE id = $1;

-- name: ListLessons :many
SELECT * FROM authoring.lesson WHERE module_id = $1 ORDER BY position, id;

-- name: UpdateLessonMetadata :one
UPDATE authoring.lesson AS l
SET title = $3, description = $4, learning_objectives = $5, estimated_duration_minutes = $6,
    revision = l.revision + 1, updated_at = now()
WHERE l.id = $1 AND l.revision = $2
  AND EXISTS (SELECT 1 FROM authoring.course_draft AS d WHERE d.id = l.draft_id AND d.status = 'ACTIVE')
RETURNING l.*;

-- name: UpdateLessonContent :one
UPDATE authoring.lesson AS l
SET content = $3, revision = l.revision + 1, updated_at = now()
WHERE l.id = $1 AND l.revision = $2
  AND EXISTS (SELECT 1 FROM authoring.course_draft AS d WHERE d.id = l.draft_id AND d.status = 'ACTIVE')
RETURNING l.*;

-- name: SetLessonPosition :one
UPDATE authoring.lesson SET position = $2, revision = revision + 1, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: SetLessonModuleAndPosition :one
UPDATE authoring.lesson SET module_id = $2, position = $3, revision = revision + 1, updated_at = now()
WHERE id = $1 AND revision = $4 RETURNING *;

-- name: DeleteLesson :one
DELETE FROM authoring.lesson AS l
WHERE l.id = $1 AND l.revision = $2
  AND EXISTS (SELECT 1 FROM authoring.course_draft AS d WHERE d.id = l.draft_id AND d.status = 'ACTIVE')
RETURNING l.id;

-- name: FindLessonByDraftAndKey :one
SELECT * FROM authoring.lesson WHERE draft_id = $1 AND stable_key = $2;

-- name: DeletePrerequisites :exec
DELETE FROM authoring.lesson_prerequisite WHERE lesson_id = $1;

-- name: AddPrerequisite :exec
INSERT INTO authoring.lesson_prerequisite (draft_id, lesson_id, prerequisite_lesson_id, position)
VALUES ($1, $2, $3, $4);

-- name: ListPrerequisites :many
SELECT p.lesson_id, p.prerequisite_lesson_id, p.position, target.stable_key AS target_stable_key
FROM authoring.lesson_prerequisite AS p
JOIN authoring.lesson AS target ON target.id = p.prerequisite_lesson_id
WHERE p.lesson_id = $1 ORDER BY p.position, p.prerequisite_lesson_id;

-- name: BumpLessonRevision :one
UPDATE authoring.lesson AS l SET revision = l.revision + 1, updated_at = now()
WHERE l.id = $1 AND l.revision = $2
  AND EXISTS (SELECT 1 FROM authoring.course_draft AS d WHERE d.id = l.draft_id AND d.status = 'ACTIVE')
RETURNING l.*;

-- name: BumpModuleRevision :one
UPDATE authoring.module AS m SET revision = m.revision + 1, updated_at = now()
WHERE m.id = $1 AND m.revision = $2
  AND EXISTS (SELECT 1 FROM authoring.course_draft AS d WHERE d.id = m.draft_id AND d.status = 'ACTIVE')
RETURNING m.*;
