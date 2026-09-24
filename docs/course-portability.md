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

## M9.1b validation boundary

Course packages are hostile input. `internal/modules/portability.Reader` accepts a bounded stream, uses a controlled temporary ZIP reader without extracting entries, removes that temporary file before returning, and performs no database, asset-storage, network, Course, or Translation writes.

v1 accepts only `bridging-the-gap-course` format version `1`. It rejects unsafe paths, duplicate entries, special ZIP files, undeclared entries, missing core files, checksum omissions, checksum references to unknown files, excessive archive/input sizes, excessive expanded entry sizes, entry counts, asset counts, translation counts, and excessive compression ratios. JSON is decoded with unknown-field, duplicate-key, and trailing-value rejection.

Validation checks every SHA-256 protected payload and asset binary, reconciles the manifest inventory with the archive, validates Course metadata/order/stable keys/prerequisites/canonical blocks, checks grading-capable Assessment bindings and KNOWLEDGE_CHECK references, requires assets to be referenced exactly as the published Course model requires, and validates complete exact-source Translation trees without accepting translation-specific grading rules. It makes no remote requests.

A successful parse yields an opaque `ValidatedCoursePackage`; only its safe `ImportPreview` is exposed for this slice. The preview has title, version, language, license, public attribution, module/lesson/assessment/asset counts, translation languages, format version, and a package digest. It deliberately omits answer keys and package binary data. ID remapping, duplicate-course policy, persistence, asset ingestion, Draft/Course creation, and any HTTP upload endpoint remain deferred to M9.1c or later.

## M9.1c1 provenance foundation

Courses distinguish `NATIVE_PUBLICATION` from `IMPORTED_PUBLICATION`. Native CourseVersions retain their existing required Draft, Review, and local publication facts. Imported CourseVersions must instead reference a portability import record that records the package fingerprint, portable source Course and CourseVersion identities, source SemVer, and import timestamp. A CourseVersion has exactly one branch; imported provenance never creates synthetic Authoring actors or history.

Assets similarly distinguish `AUTHORING_DRAFT` and `PACKAGE_IMPORT`. Native assets retain a Draft owner and upload actor. Imported assets have neither; they reference an import record and package asset key. Assets-owned `ImportIngestionService` streams and independently verifies the expected byte count and SHA-256 before it persists an `AVAILABLE` imported asset. Draft-scoped SQL explicitly selects only `AUTHORING_DRAFT` assets, while published delivery remains origin-agnostic.

PostgreSQL directly constrains the origin discriminators, Asset origin ownership, import-record fingerprints, portable source-version identities, and portable Course mappings. The requirement that an immutable imported CourseVersion has exactly its imported provenance row (and no native branch) is enforced by domain validation plus the atomic `StoreImmutableCourseVersion` transaction; it is not a standalone SQL `CHECK`, because that invariant spans tables. Repository reads reject missing or contradictory provenance.

The migration creates `portability.import_record` and `portability.source_course_mapping`. Package fingerprints are unique for future replay, while the portable CourseVersion identity is unique for future source/version collision detection.

Courses exposes a transaction-owning convenience write for native publication and a PostgreSQL-adapter `StoreImmutableCourseVersionInTx` variant for platform orchestration. Both delegate to the same immutable write sequence, including provenance, structure, content, asset bindings, and Assessment bindings. The transaction-bound variant never begins, commits, or rolls back its caller transaction. M9.1c2 will own the cross-module transaction; BinaryStorage remains outside PostgreSQL transactions and must still be staged or compensated.

Translation publications now have a corresponding explicit authoring/imported
origin. A packaged publication is stored directly against the imported local
CourseVersion with an import-record reference. It does not require a synthetic
`course_translation` workspace, local translator, or revision history. The
Translations repository validates the completed tree against that exact source
and offers `StoreImportedPublicationInTx` for the caller-owned transaction. The
ordinary learner read and language-discovery paths include these publications
without exposing import provenance. This closes the Translation provenance and
transaction boundary needed by M9.1c2; package import orchestration remains
deferred. PostgreSQL constrains the per-row origin branches and imported
source/language uniqueness. Cross-origin stream exclusion and complete tree
validation are repository/domain invariants rather than cross-row SQL checks.
