-- +goose Up
ALTER TABLE courses.course_version_publication_provenance
    ADD COLUMN published_by_user_id uuid NOT NULL;
