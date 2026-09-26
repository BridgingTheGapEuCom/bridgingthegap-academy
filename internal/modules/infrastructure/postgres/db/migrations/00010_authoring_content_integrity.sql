-- +goose Up
-- CHECK permits NULL: missing fields must explicitly fail, not evaluate unknown.
ALTER TABLE authoring.lesson DROP CONSTRAINT authoring_lesson_content;
ALTER TABLE authoring.lesson ADD CONSTRAINT authoring_lesson_content CHECK (
    (jsonb_typeof(content) = 'object'
     AND jsonb_typeof(content -> 'schemaVersion') = 'number'
     AND content ->> 'schemaVersion' = '1'
     AND jsonb_typeof(content -> 'blocks') = 'array') IS TRUE
);
