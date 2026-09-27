-- +goose Up
-- BCP 47 grammar and canonical casing are enforced by the shared Courses
-- domain utility. PostgreSQL keeps only inexpensive invariants so valid tags
-- are not limited to a hand-maintained regular-expression subset.
ALTER TABLE courses.course_version DROP CONSTRAINT course_version_language;
ALTER TABLE courses.course_version ADD CONSTRAINT course_version_language CHECK (source_language = btrim(source_language) AND char_length(source_language) BETWEEN 1 AND 64);

ALTER TABLE authoring.course_draft DROP CONSTRAINT authoring_draft_language;
ALTER TABLE authoring.course_draft ADD CONSTRAINT authoring_draft_language CHECK (source_language = btrim(source_language) AND char_length(source_language) BETWEEN 1 AND 64);

ALTER TABLE translations.course_translation ADD CONSTRAINT translation_source_language_trimmed CHECK (source_language = btrim(source_language));
ALTER TABLE translations.course_translation ADD CONSTRAINT translation_target_language_trimmed CHECK (target_language = btrim(target_language));
ALTER TABLE translations.translation_publication ADD CONSTRAINT translation_publication_source_language_trimmed CHECK (source_language = btrim(source_language));
ALTER TABLE translations.translation_publication ADD CONSTRAINT translation_publication_target_language_trimmed CHECK (target_language = btrim(target_language));

-- +goose Down
ALTER TABLE translations.translation_publication DROP CONSTRAINT translation_publication_target_language_trimmed;
ALTER TABLE translations.translation_publication DROP CONSTRAINT translation_publication_source_language_trimmed;
ALTER TABLE translations.course_translation DROP CONSTRAINT translation_target_language_trimmed;
ALTER TABLE translations.course_translation DROP CONSTRAINT translation_source_language_trimmed;

ALTER TABLE authoring.course_draft DROP CONSTRAINT authoring_draft_language;
ALTER TABLE authoring.course_draft ADD CONSTRAINT authoring_draft_language CHECK (source_language ~ '^[a-z]{2,3}(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?(-[a-z0-9]{2,8})*$' AND char_length(source_language) <= 64);

ALTER TABLE courses.course_version DROP CONSTRAINT course_version_language;
ALTER TABLE courses.course_version ADD CONSTRAINT course_version_language CHECK (source_language ~ '^[a-z]{2,3}(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?(-[a-z0-9]{2,8})*$' AND char_length(source_language) <= 64);
