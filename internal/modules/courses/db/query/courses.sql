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
