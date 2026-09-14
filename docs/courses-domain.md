# Courses domain

`Course` is the stable, URL-addressable container. Its immutable canonical slug does not carry learner-facing content.

`CourseVersion` is an immutable published learning artifact. Its title, description, ordered objectives, source language, changelog, content license, and contributor attribution belong to that version and have no general update operation. The only mutable operational field is its lifecycle status: `PUBLISHED`, `DEPRECATED`, `ARCHIVED`, or `WITHDRAWN`.

Versions use a constrained SemVer-like `major.minor.patch` form. The database keeps parsed numeric components for deterministic ordering and prevents duplicate versions within a Course.

Content licensing is version-owned and separate from the Academy software license. Attribution is an ordered version-owned JSONB snapshot so historical authorship remains available without querying Identity; later Publishing supplies those snapshots.
