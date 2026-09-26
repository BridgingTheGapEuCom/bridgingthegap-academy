-- +goose Up
ALTER TABLE translations.translation_publication
    ADD COLUMN publication_origin text NOT NULL DEFAULT 'AUTHORING_PUBLICATION'
        CHECK (publication_origin IN ('AUTHORING_PUBLICATION', 'IMPORTED_PUBLICATION')),
    ADD COLUMN import_id uuid REFERENCES portability.import_record(id) ON DELETE RESTRICT;
ALTER TABLE translations.translation_publication ALTER COLUMN translation_id DROP NOT NULL;
ALTER TABLE translations.translation_publication ALTER COLUMN translation_revision DROP NOT NULL;
ALTER TABLE translations.translation_publication ADD CONSTRAINT translation_publication_origin_provenance CHECK (
    (publication_origin = 'AUTHORING_PUBLICATION' AND translation_id IS NOT NULL AND translation_revision IS NOT NULL AND import_id IS NULL)
    OR (publication_origin = 'IMPORTED_PUBLICATION' AND translation_id IS NULL AND translation_revision IS NULL AND import_id IS NOT NULL)
);
CREATE UNIQUE INDEX translation_import_publication_identity ON translations.translation_publication(import_id, target_language)
    WHERE publication_origin = 'IMPORTED_PUBLICATION';
CREATE UNIQUE INDEX translation_import_source_language ON translations.translation_publication(source_course_version_id, target_language)
    WHERE publication_origin = 'IMPORTED_PUBLICATION';
DROP INDEX translations.translation_publication_latest_idx;
CREATE INDEX translation_publication_latest_idx ON translations.translation_publication(source_course_version_id, target_language, translation_revision DESC NULLS LAST, published_at DESC, id DESC);

-- +goose Down
DROP INDEX translations.translation_publication_latest_idx;
CREATE INDEX translation_publication_latest_idx ON translations.translation_publication(source_course_version_id, target_language, translation_revision DESC, id DESC);
DROP INDEX translations.translation_import_source_language;
DROP INDEX translations.translation_import_publication_identity;
ALTER TABLE translations.translation_publication DROP CONSTRAINT translation_publication_origin_provenance;
ALTER TABLE translations.translation_publication DROP COLUMN import_id;
ALTER TABLE translations.translation_publication DROP COLUMN publication_origin;
ALTER TABLE translations.translation_publication ALTER COLUMN translation_id SET NOT NULL;
ALTER TABLE translations.translation_publication ALTER COLUMN translation_revision SET NOT NULL;
