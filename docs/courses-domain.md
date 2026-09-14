# Courses domain

`Course` is the stable, URL-addressable container. Its immutable canonical slug does not carry learner-facing content.

`CourseVersion` is an immutable published learning artifact. Its title, description, ordered objectives, source language, changelog, content license, and contributor attribution belong to that version and have no general update operation. The only mutable operational field is its lifecycle status: `PUBLISHED`, `DEPRECATED`, `ARCHIVED`, or `WITHDRAWN`.

Versions use a constrained SemVer-like `major.minor.patch` form. The database keeps parsed numeric components for deterministic ordering and prevents duplicate versions within a Course.

Content licensing is version-owned and separate from the Academy software license. Attribution is an ordered version-owned JSONB snapshot so historical authorship remains available without querying Identity; later Publishing supplies those snapshots.

Each `CourseVersion` owns ordered `Module` rows, and each Module owns ordered `Lesson` rows. Positions are zero-based and explicit. Module keys are unique within a CourseVersion; Lesson keys are also unique within a CourseVersion, which lets separate Lesson rows in later versions express logical correspondence for future progress transfer without sharing mutable identity.

Lesson objectives are ordered JSONB metadata. Estimated duration is optional, expressed in positive minutes, and is informational only. Recommended prerequisites are ordered same-version Lesson references: they are advisory, never access-control gates. Lessons deliberately have no difficulty field. Modules, Lessons, and prerequisites have no general update or delete operation after publication.

Each Lesson also owns one presentation-independent JSONB content document: `{ "schemaVersion": 1, "blocks": [] }`. Array order is canonical for both future Continuous and Focus rendering; each block has an immutable lesson-local stable key. The built-in semantic block types are `TEXT`, `HEADING`, `IMAGE`, `VIDEO`, `AUDIO`, `CODE`, `QUOTE`, `CALLOUT`, `TABLE`, `DOWNLOAD`, `KNOWLEDGE_CHECK`, and `DIVIDER`. The document uses a constrained rich-text model rather than HTML or editor state. Image alt/decorative rules, transcript/caption media metadata, table headers, heading levels, safe links, and bounded sizes are validated before persistence. Empty documents are retained only as a migration-compatible structural state for M2.2 rows; new publication should provide blocks.

The content schema version is technical storage format versioning, separate from CourseVersion SemVer. Unknown versions and block types fail closed. External widgets, asset storage, Tiptap, rendering, and assessment resolution remain post-v1 or later milestones.
