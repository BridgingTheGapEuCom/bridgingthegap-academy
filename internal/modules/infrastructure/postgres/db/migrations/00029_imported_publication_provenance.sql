-- +goose Up
CREATE SCHEMA IF NOT EXISTS portability;
CREATE TABLE portability.import_record (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 package_fingerprint text NOT NULL UNIQUE CHECK (package_fingerprint ~ '^[0-9a-f]{64}$'),
 portable_source_course_id text NOT NULL,
 portable_source_course_version_id text NOT NULL,
 source_version text NOT NULL,
 target_course_id uuid,
 target_course_version_id uuid,
 imported_at timestamptz NOT NULL,
 UNIQUE (portable_source_course_id, portable_source_course_version_id)
);
CREATE TABLE portability.source_course_mapping (
 portable_source_course_id text PRIMARY KEY,
 local_course_id uuid NOT NULL REFERENCES courses.course(id) ON DELETE RESTRICT
);
ALTER TABLE courses.course_version ADD COLUMN publication_origin text NOT NULL DEFAULT 'NATIVE_PUBLICATION' CHECK (publication_origin IN ('NATIVE_PUBLICATION','IMPORTED_PUBLICATION'));
CREATE TABLE courses.course_version_import_provenance (
 course_version_id uuid PRIMARY KEY REFERENCES courses.course_version(id) ON DELETE RESTRICT,
 import_id uuid NOT NULL UNIQUE REFERENCES portability.import_record(id) ON DELETE RESTRICT
);
ALTER TABLE assets.asset ADD COLUMN origin text NOT NULL DEFAULT 'AUTHORING_DRAFT' CHECK (origin IN ('AUTHORING_DRAFT','PACKAGE_IMPORT'));
ALTER TABLE assets.asset ADD COLUMN import_id uuid REFERENCES portability.import_record(id) ON DELETE RESTRICT;
ALTER TABLE assets.asset ALTER COLUMN owner_draft_id DROP NOT NULL;
ALTER TABLE assets.asset ALTER COLUMN created_by_user_id DROP NOT NULL;
ALTER TABLE assets.asset ADD CONSTRAINT asset_origin_provenance CHECK ((origin='AUTHORING_DRAFT' AND owner_draft_id IS NOT NULL AND created_by_user_id IS NOT NULL AND import_id IS NULL) OR (origin='PACKAGE_IMPORT' AND owner_draft_id IS NULL AND created_by_user_id IS NULL AND import_id IS NOT NULL));
-- +goose Down
ALTER TABLE assets.asset DROP CONSTRAINT asset_origin_provenance;
ALTER TABLE assets.asset DROP COLUMN import_id;
ALTER TABLE assets.asset DROP COLUMN origin;
ALTER TABLE assets.asset ALTER COLUMN owner_draft_id SET NOT NULL;
ALTER TABLE assets.asset ALTER COLUMN created_by_user_id SET NOT NULL;
DROP TABLE courses.course_version_import_provenance;
ALTER TABLE courses.course_version DROP COLUMN publication_origin;
DROP TABLE portability.source_course_mapping;
DROP TABLE portability.import_record;
