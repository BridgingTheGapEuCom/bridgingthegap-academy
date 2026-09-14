-- +goose Up
ALTER TABLE courses.lesson
    ADD COLUMN content jsonb NOT NULL DEFAULT '{"schemaVersion": 1, "blocks": []}'::jsonb;

ALTER TABLE courses.lesson
    ALTER COLUMN content DROP DEFAULT,
    ADD CONSTRAINT course_lesson_content_shape CHECK (
        jsonb_typeof(content) = 'object'
        AND jsonb_typeof(content -> 'schemaVersion') = 'number'
        AND content ->> 'schemaVersion' = '1'
        AND jsonb_typeof(content -> 'blocks') = 'array'
    );
