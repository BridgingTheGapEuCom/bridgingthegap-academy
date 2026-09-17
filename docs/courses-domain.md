# Courses domain

`Course` is the stable, URL-addressable container. Its immutable canonical slug does not carry learner-facing content.

`CourseVersion` is an immutable published learning artifact. Its title, description, ordered objectives, source language, changelog, content license, and contributor attribution belong to that version and have no general update operation. The only mutable operational field is its lifecycle status: `PUBLISHED`, `DEPRECATED`, `ARCHIVED`, or `WITHDRAWN`.

Versions use a constrained SemVer-like `major.minor.patch` form. The database keeps parsed numeric components for deterministic ordering and prevents duplicate versions within a Course.

Content licensing is version-owned and separate from the Academy software license. Attribution is an ordered version-owned JSONB snapshot so historical authorship remains available without querying Identity; later Publishing supplies those snapshots.

Each `CourseVersion` owns ordered `Module` rows, and each Module owns ordered `Lesson` rows. Positions are zero-based and explicit. Module keys are unique within a CourseVersion; Lesson keys are also unique within a CourseVersion, which lets separate Lesson rows in later versions express logical correspondence for future progress transfer without sharing mutable identity.

Lesson objectives are ordered JSONB metadata. Estimated duration is optional, expressed in positive minutes, and is informational only. Recommended prerequisites are ordered same-version Lesson references: they are advisory, never access-control gates. Lessons deliberately have no difficulty field. Modules, Lessons, and prerequisites have no general update or delete operation after publication. Future Publishing must create the version and its full structure in one transaction before exposing it; the current internal create operations do not themselves define a publication workflow.

Each Lesson also owns one presentation-independent JSONB content document: `{ "schemaVersion": 1, "blocks": [] }`. Array order is canonical for both Continuous and Focus rendering; each block has an immutable lesson-local stable key. The built-in semantic block types are `TEXT`, `HEADING`, `IMAGE`, `VIDEO`, `AUDIO`, `CODE`, `QUOTE`, `CALLOUT`, `TABLE`, `DOWNLOAD`, `KNOWLEDGE_CHECK`, and `DIVIDER`. The document uses a constrained rich-text model rather than HTML or editor state. Image alt/decorative rules, transcript/caption media metadata, table headers, heading levels, safe links, and bounded sizes are validated before persistence. Empty documents remain a supported migration-compatible structural state and render as an explicit empty state without Focus controls.

Publishing persistence accepts one already-built `ImmutableCourseVersion` and
stores its version, Review/Draft provenance, source identifiers, ordered
structure, prerequisites, and canonical content in one Courses-owned database
transaction. Course identity plus SemVer is unique, and Review ID is separately
unique to prevent replay under another version. Any parent, child, provenance,
or prerequisite failure rolls the transaction back. Once stored, the immutable
aggregate can be reconstructed entirely from Courses without consulting
Authoring. Courses also retains the publication actor so cross-module recovery
can recreate an Authoring publication fact from the already-committed immutable
aggregate. Authoring orchestration treats an exact Review-provenance replay as
reconciliation while preserving a Course plus SemVer collision owned by another
Review as a conflict. Review mutation and public publication APIs remain separate
work.

The content schema version is technical storage format versioning, separate from CourseVersion SemVer. Unknown versions and block types fail closed. External widgets, asset storage, Tiptap, rendering, and assessment resolution remain post-v1 or later milestones.

The public learner read API is read-only. Discovery and `/api/courses/{slug}` select the highest numeric SemVer version whose status is `PUBLISHED`; `DEPRECATED` and `ARCHIVED` versions never become preferred or appear in ordinary discovery. Explicit version routes may serve `PUBLISHED`, `DEPRECATED`, and `ARCHIVED` artifacts for historical learning/provenance. `WITHDRAWN` artifacts deliberately return the same public not-found response as unavailable content and never serve course or lesson data. Course outlines query lesson metadata only; the exact lesson endpoint loads its block document. All course responses use `Cache-Control: no-store`: artifact content is immutable, but its serving eligibility can change immediately on withdrawal. Deployments that previously cached public course responses must purge those caches when withdrawing a version; response headers cannot revoke copies already stored.

The immutable publication read boundary is Courses-owned: `/api/courses/by-id/{courseId}/versions/{version}` returns one complete `PUBLISHED` aggregate and `/api/courses/by-id/{courseId}/latest` returns the highest constrained SemVer. These reads reconstruct one CourseVersion and all of its children only from Courses tables, never Authoring Drafts, Reviews, or publication facts. The public projection retains canonical content, ordering, asset and assessment references, and public attribution while excluding Review/Draft/snapshot provenance and storage identifiers. The `by-id` namespace preserves the existing slug-addressed learner URLs; the current SemVer model intentionally has no prerelease metadata.

`/api/courses/catalog` is also Courses-owned and returns a paginated lightweight summary for each logical Course with a complete immutable `PUBLISHED` version. It selects one latest version per Course by numeric SemVer, then orders summaries by title and Course ID. Its optional normalized source-language filter applies to that selected latest version. The catalog never consults unpublished Authoring state or exposes Review/Draft provenance, modules, lessons, or canonical content; consumers fetch a selected full version through the immutable `by-id` read API.

The published course reader renders canonical LessonContent from the exact loaded CourseVersion through the shared closed block renderer. It rejects Course/version payloads that do not match the active route, strictly normalizes Lesson selection within that version, and never substitutes Authoring or Draft data. The renderer never executes authored HTML. Unresolved media and download assets remain descriptive read-only placeholders until asset delivery exists; knowledge checks remain non-interactive placeholders until assessment delivery exists. Enrollment, progress, and completion are not part of the reader.

The learner catalog is discovery data only; the reader always fetches its own exact Courses aggregate, and the backend alone resolves `latest`. Catalog and reader requests bypass browser storage and HTTP reuse so a withdrawn publication is not retained as learner content. The frontend rejects malformed or route-mismatched public responses, exact-version routes never fall back to latest, and an out-of-range catalog page is normalized after publication visibility changes.
