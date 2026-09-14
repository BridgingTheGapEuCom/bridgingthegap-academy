# Authoring domain (M3.1)

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
orchestration are separate future boundaries. Authoring currently imports only
Courses' canonical domain value objects and validation, not Courses persistence.
