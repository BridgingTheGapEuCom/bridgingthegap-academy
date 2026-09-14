-- name: CreateCourse :one
INSERT INTO courses.course (slug)
VALUES ($1)
RETURNING *;

-- name: GetCourse :one
SELECT *
FROM courses.course
WHERE id = $1;

-- name: GetCourseBySlug :one
SELECT *
FROM courses.course
WHERE slug = $1;

-- name: ListCourses :many
SELECT *
FROM courses.course
ORDER BY slug ASC, id ASC;

-- name: CreateCourseVersion :one
INSERT INTO courses.course_version (
    course_id, version, status, title, description, learning_objectives,
    source_language, changelog, license_kind, license_identifier,
    license_display_name, license_url, license_custom_text, attribution, published_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: GetCourseVersion :one
SELECT *
FROM courses.course_version
WHERE id = $1;

-- name: GetCourseVersionByCourseAndVersion :one
SELECT *
FROM courses.course_version
WHERE course_id = $1 AND version = $2;

-- name: ListCourseVersions :many
SELECT *
FROM courses.course_version
WHERE course_id = $1
ORDER BY version_major DESC, version_minor DESC, version_patch DESC, id DESC;

-- name: TransitionCourseVersionStatus :one
UPDATE courses.course_version
SET status = $3
WHERE id = $1 AND status = $2
RETURNING *;

-- name: CreateModule :one
INSERT INTO courses.module (course_version_id, stable_key, title, description, position)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetModule :one
SELECT *
FROM courses.module
WHERE id = $1;

-- name: GetModuleByCourseVersionAndKey :one
SELECT *
FROM courses.module
WHERE course_version_id = $1 AND stable_key = $2;

-- name: ListModulesForCourseVersion :many
SELECT *
FROM courses.module
WHERE course_version_id = $1
ORDER BY position ASC, id ASC;

-- name: CreateLesson :one
INSERT INTO courses.lesson (
    course_version_id, module_id, stable_key, title, description,
    learning_objectives, estimated_duration_minutes, position
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetLesson :one
SELECT *
FROM courses.lesson
WHERE id = $1;

-- name: GetLessonByCourseVersionAndKey :one
SELECT *
FROM courses.lesson
WHERE course_version_id = $1 AND stable_key = $2;

-- name: ListLessonsForModule :many
SELECT *
FROM courses.lesson
WHERE module_id = $1
ORDER BY position ASC, id ASC;

-- name: CreateLessonPrerequisite :one
INSERT INTO courses.lesson_prerequisite (course_version_id, lesson_id, prerequisite_lesson_id, position)
VALUES ($1, $2, $3, $4)
RETURNING course_version_id, lesson_id, prerequisite_lesson_id, position;

-- name: ListLessonPrerequisites :many
SELECT
    prerequisite.course_version_id,
    prerequisite.lesson_id,
    prerequisite.prerequisite_lesson_id,
    prerequisite.position,
    target.stable_key AS prerequisite_stable_key
FROM courses.lesson_prerequisite AS prerequisite
JOIN courses.lesson AS target ON target.id = prerequisite.prerequisite_lesson_id
WHERE prerequisite.lesson_id = $1
ORDER BY prerequisite.position ASC, prerequisite.prerequisite_lesson_id ASC;
