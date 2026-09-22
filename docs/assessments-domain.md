# Assessments domain

`assessments` is a neutral module for mutable, deterministic Assessment
definitions. It owns Assessment identity, Draft ownership, creator provenance,
question validation, and persistence. It does not import Authoring or Courses;
Authoring will later authorize Draft access before calling it, and Courses will
later own an immutable published assessment representation.

An Assessment has an opaque lowercase UUID identity. Its `owner_draft_id` and
`created_by_user_id` are internal provenance fields, never default public JSON.
An Assessment has an explicit monotonic revision for conditional updates. An
empty question list is valid mutable Authoring state; publication readiness is a
separate future concern.

The supported deterministic question types are `SINGLE_CHOICE`,
`MULTIPLE_CHOICE`, and one-to-one `MATCHING`. Questions, choices, and matching
items use stable local keys. Explicit positions define display order, so
reordering does not change identity or correct-answer references. Correct
answers reference stable keys and are retained in the internal aggregate and
database definition, but domain defaults deliberately do not serialize
questions or answer keys for learner-facing use. Future Authoring and learner
DTOs must select presentation fields explicitly.

PostgreSQL stores each aggregate atomically in `assessments.assessment`. The
metadata row contains the owner, title, revision, provenance, and timestamps;
the validated deterministic question definition is one JSONB value. Conditional
updates replace that definition and increment the revision in one statement, so
callers cannot observe a partial question set.

Authoring manages Assessments through private Draft-scoped create, list, exact
read, and aggregate-replacement update routes. They require
`authoring.assessment.edit`, granted to active AUTHOR and MAINTAINER members;
global administrators have no implicit Draft bypass. Draft and creator values
come only from the route and resolved session. The list is bounded and ordered
by `updatedAt DESC, assessmentKey DESC`; it returns summaries without answer
keys. Exact Authoring detail returns a deliberately named answer-bearing DTO
for editing. Neither response exposes owner or creator provenance, and neither
DTO may be reused for a learner API.

Canonical `KNOWLEDGE_CHECK.assessmentKey` uses the lowercase UUID format used
by Assessment IDs. Review submission resolves those keys inside the same
repeatable-read PostgreSQL transaction as the Draft snapshot and privately
freezes each exact Draft-owned definition. Malformed, missing, and foreign
references remain unfrozen and later produce the same safe unavailable
publication issue. An empty mutable Assessment is frozen faithfully but is not
publication-ready.

Assessment deletion, cross-Draft reuse, free-form grading, and external
assessment engines are deliberately unsupported. Publication maps the private
Review copy into a Courses-owned immutable binding; it never reads the current
Assessment. Questions, stable keys, order, and authoritative answers are
copied, while Draft owner, creator, revision, and Authoring timestamps are
excluded. Answers remain server-private and are not part of public Course
projections. Exact CourseVersion reads map the binding to an answer-free learner
projection containing only semantic keys, frozen presentation text, and order.
The learner reader supports unsaved, ungraded local practice responses; attempts
and grading remain deferred.

`assessments.assessment_attempt` now owns private persisted learner response
aggregates. Every Attempt has its own opaque UUID, one durable Identity user,
one exact internal CourseVersion UUID, and one canonical `assessmentKey`. A
composite foreign key requires that key to exist in the exact Courses-owned
published Assessment binding, so an Attempt cannot silently drift to another
CourseVersion or mutable Authoring Assessment. There is intentionally no
foreign key to `assessments.assessment`.

Attempts begin as `IN_PROGRESS` and may contain no responses while a future API
is saving progress. Responses use stable question, option, and matching-item
keys: a single selected option, a canonical option-key set, or canonical
one-to-one matching pairs. The stored JSONB response document is validated on
write and read; it never includes correct answers, grading data, or feedback.
Revision-based compare-and-swap protects `IN_PROGRESS` updates. `SUBMITTED`
sets a server-owned timestamp and is immutable. Submission and grading HTTP
APIs remain separate work, and anonymous learner practice remains in memory
until a durable authenticated learner identity is available at that boundary.

The Draft Authoring workspace provides private Assessment list and editor
routes. The editor uses native radios, checkboxes, selects, and move controls;
stable question, option, and matching-item keys survive text edits and
reordering. Assessment saves replace the complete aggregate using its exact
revision. A revision conflict preserves local edits and requires an explicit
reload; it is never merged or retried automatically. Correct answers appear
only in this authenticated editor, never in Assessment lists or Lesson block
choosers.
