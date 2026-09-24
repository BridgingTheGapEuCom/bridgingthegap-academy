# Course portability

M9.1a defines the Academy Course Package v1 (`bridging-the-gap-course`, format version `1`). It exports one exact immutable published CourseVersion. It never selects the latest version, reads Authoring Drafts or Reviews, or exports learner records.

## Archive layout

```
manifest.json
course.json
assessments.json
translations/<language>.json
assets/<asset-key>/content
checksums.json
```

`manifest.json` identifies the format, exact source SemVer and language, frozen license and public attribution, a content inventory, included assets, and TranslationPublications. `checksums.json` contains SHA-256 hashes for each payload and asset entry; it intentionally does not hash the manifest to avoid a circular checksum. Checksums detect accidental corruption and are not a publisher-authenticity signature.

`course.json` is canonical structured Course content: modules, lessons, prerequisites, and `LessonContent` blocks. Stable module, lesson, block, assessment, question, option, and matching-item keys remain portable semantic references. It has no database row IDs. `assessments.json` contains the immutable authoritative assessment binding, including correct answers and pairs. Packages are trusted administrative artifacts and must never be served through learner/public endpoints.

Asset archive paths are generated solely from the frozen asset key. Original filenames are metadata only. The exporter reads each binary through `BinaryStorage`, checks its byte count and SHA-256 against the published binding before the archive is started, then streams it into the ZIP. Storage object IDs are never serialized.

Published TranslationPublications are included only when their exact bound source CourseVersion is the exported version. Translation drafts and publications for older or newer source versions are excluded. Their trees retain source stable-key mappings and only learner-visible text; source assessment grading semantics remain in `assessments.json`.

## Identity and privacy

The manifest retains origin Course and CourseVersion UUIDs as optional provenance hints. They are not package-local identity requirements: a future importer must create installation-local records. Public attribution includes display name, role, and order only. The package excludes Draft/Review provenance, contributor Identity UUIDs, audit actors, storage IDs, authentication data, learner attempts/progress, Community posts, Certificates, Open Badges, and secrets.

## Determinism and failure behavior

For the same immutable inputs and injected `exportedAt`, logical JSON payloads, checksums, ordering, and generated paths are deterministic. ZIP headers use a fixed timestamp; no package signing or authenticity claim is made. Asset bytes are verified before any archive entry is written, so a corrupt frozen binding fails the export rather than producing a successful package. A caller must treat any writer error as an incomplete export; HTTP export and import are deferred.

## v1 boundaries

No import path, export HTTP endpoint, mutable Draft export, learner data migration, Community export, certificate export, package encryption, or package signing exists in M9.1a. A future importer must validate this strict versioned contract before it creates any local Course records.
