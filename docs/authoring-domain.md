# Authoring domain

**A draft is a workspace; a published CourseVersion is an immutable artifact.**
Authoring owns current mutable content in its own PostgreSQL schema. Its `course_id`
references the stable Course container, but no Authoring operation writes Courses
tables or creates a published version. Multiple drafts may target the same intended
SemVer; Publishing must later recheck version uniqueness and create new published
row IDs atomically.

One CourseDraft has one AuthoringWorkspace. The workspace records its creator and
last content activity. Membership keeps historical MAINTAINER/AUTHOR assignments,
with at most one active assignment per user. User IDs are opaque references; there
is no Identity foreign key or permission decision in this milestone.

Draft Modules and Lessons have explicit, database-unique positions and stable keys.
Lesson keys are unique across the entire draft, so moving between Modules does not
change logical identity. These keys, plus block keys inside canonical LessonContent,
are intended to carry through the later Draft-to-CourseVersion conversion. Published
rows will have different artifact IDs. LessonContent uses the Courses schema-v1
semantic document and validator, with no editor transactions, cursor state, or
comments. Empty valid content can exist in a draft; Publishing must validate its
own completeness rules.

Draft, Module, and Lesson revisions start at 1. Expected-revision updates reject
stale writes; child mutations also advance the draft revision. Reorders and moves
lock the draft, defer position uniqueness only within the transaction, and commit
a complete new order. Prerequisites are same-draft relational, ordered, advisory,
and use target stable keys in the domain API. Draft deletion is hard deletion:
deleting a lesson or Module cascades to its draft-owned children and prerequisite
references. No published CourseVersion deletion or mutation path is added.

Locks, autosave, revision history, comments, review states, and publication
orchestration are separate future boundaries. Authoring imports Courses'
canonical value objects and Identity's trusted actor type, never their
persistence adapters.

Authorization uses a current, active membership for the exact Draft/workspace.
Future handlers must ask the Authoring authorizer for a capability and must not
branch on MAINTAINER/AUTHOR roles. The policy is:

| Capability | AUTHOR | MAINTAINER |
| --- | --- | --- |
| `authoring.read` | Allow | Allow |
| `authoring.draft.edit` | Allow | Allow |
| `authoring.structure.edit` | Allow | Allow |
| `authoring.content.edit` | Allow | Allow |
| `authoring.members.manage` | Deny | Allow |
| `authoring.draft.abandon` | Deny | Allow |

Membership is read on every authorization decision; revoked rows grant nothing.
An in-flight check may observe the prior committed membership if it races with
revocation; the next check after the revoke commits sees the new state.
Missing membership and missing Draft both deny, while a database failure returns
authorization unavailable. There is no implicit global ADMINISTRATOR bypass.
The future HTTP layer should keep authentication, authorization, resource
existence, and draft-lifecycle checks distinct, and may collapse missing or
unauthorized resources to one outward response where enumeration matters.

The private read API exposes Draft metadata, workspace metadata, ordered draft
structure, and individual canonical LessonContent only to an authenticated
member with `authoring.read` on that exact Draft. It uses `404 Not Found` for
both absent and unauthorized Draft resources, while malformed identifiers are
`400` and authorization or storage failures are `500`. Responses are always
`Cache-Control: no-store`: drafts are mutable and private. Current revision
numbers are explicit in DTOs for later optimistic writes; no member list or
Identity profile data is included in Draft metadata or structure responses.
`GET /api/authoring/drafts/{draftId}/members` is a separate `authoring.read`,
`no-store` projection of only active opaque user IDs and AUTHOR/MAINTAINER
roles, ordered by user ID. It never exposes revoked membership history.

The metadata PATCH endpoint accepts only intended version, source language,
title, description, objectives, changelog, and content license. It requires an
explicit expected revision and rejects a no-op patch. The Authoring mutation
service composes a validated complete metadata value and delegates to the
repository compare-and-swap update; a stale revision returns `409` without
overwriting committed work. Browser writes inherit the established trusted
Origin and session-bound CSRF checks, and retain the private-resource `404`
hiding policy. Audit is deliberately deferred: the existing audit transaction
orchestrators are scoped to session lifecycle work, and this slice does not add
an Authoring-to-Audit persistence dependency without a shared transactional
mutation boundary.

Draft Module mutations require `authoring.structure.edit`, the trusted Origin,
and the session-bound CSRF token. Creation, full-list reorder, and deletion use
the current Draft revision; metadata PATCH uses the Module revision and also
advances the Draft revision. Module stable keys are immutable through ordinary
metadata PATCH because they carry semantic identity into publication. Reorder
accepts every Module ID exactly once and commits contiguous zero-based
positions atomically. The API deliberately deletes only empty Modules: a
Module containing Lessons returns a conflict, rather than exposing the
persistence cascade through an ordinary structural request. As with all
private Authoring responses, mutation results are `Cache-Control: no-store`.

Draft Lesson structural and metadata mutations also require
`authoring.structure.edit`, the trusted Origin, and the session-bound CSRF
token. A Lesson is created with an empty valid canonical content document.
Ordinary metadata PATCH accepts only title, description, objectives, and
estimated duration; stable keys, Module assignment, position, prerequisites,
and content each remain outside that operation. Lesson stable keys are immutable
semantic identities across the whole Draft and survive moves between Modules.

Lesson ordering is replaced as one complete Draft-wide Module-to-Lesson layout.
It contains every current Module and Lesson exactly once, so moves and
zero-based contiguous positions commit atomically. The Draft revision advances
once; moved Lessons and affected Modules advance their revisions so clients can
observe the structural change. Prerequisites are atomically replaced as one
ordered, same-Draft stable-key list. They remain advisory: direct self-links
and duplicates are rejected, while general cycles are not evaluated in this
slice. Deleting a Lesson is a hard delete. Incoming prerequisite rows are
explicitly removed and their source Lesson revisions advance before the target
is removed; remaining Lessons in its Module are compacted. These structural
mutations are future audit candidates, but audit integration remains deferred
until a shared transaction boundary exists.

Draft LessonContent replacement is a separate `authoring.content.edit` boundary.
It accepts the complete canonical Courses `LessonContent` document and an
expected Lesson revision, validates the document with the shared semantic model,
then atomically replaces it. A successful replacement advances both the Lesson
and Draft revisions; a stale Lesson revision conflicts without overwriting the
committed document. The endpoint is private, `no-store`, and inherits the
trusted-Origin and session-bound CSRF requirements. It deliberately offers no
block-level editing, editor-state persistence, autosave, locks, history, or
assessment behavior.

Membership changes are a separate `authoring.members.manage` boundary. Adding,
changing a role, and revoking a member all require the current Draft revision
and commit that revision once with the membership change. Roles are limited to
AUTHOR and MAINTAINER. Role changes preserve membership history by revoking the
previous active row and creating a new active row. Revocation preserves the
historical row. Authoring does not query Identity: member user IDs are opaque
UUID references. Every mutation is serialized by the Draft transaction and
will reject a demotion or revocation that would leave no active MAINTAINER.

The Authoring Members page uses that active-members projection as its sole
membership source. It uses opaque IDs without an Identity lookup, reloads the
server list after every successful add, role change, or revocation, and keeps
the server-returned Draft revision for later writes. Keyboard-accessible role
and revoke controls remain advisory to the backend authorization boundary; a
membership conflict is never retried or merged automatically.

The complete API shares one bounded strict JSON decoder: duplicate members,
unknown or incorrectly cased request fields, trailing data, and invalid nulls
are rejected. Reorder arrays must be explicitly supplied (an empty array is
valid for an empty structure). Full Lesson layouts have a 1 MiB body budget,
large enough for the existing 1,000-Module / 10,000-Lesson limits; smaller
metadata and membership bodies retain their narrower budgets.

Outline and individual Lesson reads use a single read-only PostgreSQL snapshot
for metadata, ordering, revisions, and prerequisites. Outline/reorder queries
project Lesson metadata without loading every canonical content document.
Empty prerequisite lists are JSON arrays, never null. Lesson insertion and
deletion advance the owning Module revision. Deletion advances each affected
source/shifted Lesson exactly once, even when it both loses a prerequisite and
changes position. Failed writes roll back every revision and structural change.
All membership repository writes require a Draft revision; there is no legacy
unrevisioned add/revoke path that can bypass last-maintainer protection.

A forward-only migration makes the draft content top-level CHECK fail on
missing/null fields and require a numeric schema version. Valid empty draft
documents remain supported; detailed semantic validation stays in the canonical
Courses value object.
