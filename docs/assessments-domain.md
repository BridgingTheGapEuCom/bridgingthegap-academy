# Assessments domain

`assessments` is a neutral module for mutable, deterministic Assessment
definitions. It owns Assessment identity, Draft ownership, creator provenance,
question validation, and persistence. It does not import Authoring. Its learner
Attempt service reads only the narrow Courses-owned immutable CourseVersion
binding boundary for validation and grading; Courses never imports Attempts or
mutable Authoring Assessments.

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
write and read; it never includes correct answers or feedback. Revision-based
compare-and-swap protects `IN_PROGRESS` updates.

Authenticated learner APIs create an Attempt only through an exact published
Course ID, SemVer, and bound `assessmentKey`; the learner identity always comes
from the server session. Reads, response replacement, and submission verify
that same learner ownership and hide other learners' Attempts as not found.
All Attempt routes use the normal session Origin/CSRF protections and
`Cache-Control: no-store`. Anonymous public-course practice remains in memory;
it never creates persistent Attempt state.

An IN_PROGRESS update may be partial. Submission requires one response for
every frozen question, a selection for choice questions, and a complete mapping
for every MATCHING left item. The server validates those response keys against
the exact immutable Courses binding, then grades single choice by equality,
multiple choice by exact key-set equality, and matching by exact one-to-one
mapping. Each question has equal weight and there is no partial credit. Wrong
answers are valid submissions, not validation failures.

`SUBMITTED` is terminal. Its single atomic CAS update writes the server-owned
submission time plus the private aggregate `correctCount` and `totalCount`;
there can be no submitted Attempt without a result. The learner DTO exposes
only that aggregate result and the learner's own responses. It never exposes
correct options, correct sets, matching answers, mutable Authoring data, or
learner identity. Repeated submission returns a conflict rather than grading
again. Learner UI wiring and detailed feedback remain deferred.

The Draft Authoring workspace provides private Assessment list and editor
routes. The editor uses native radios, checkboxes, selects, and move controls;
stable question, option, and matching-item keys survive text edits and
reordering. Assessment saves replace the complete aggregate using its exact
revision. A revision conflict preserves local edits and requires an explicit
reload; it is never merged or retried automatically. Correct answers appear
only in this authenticated editor, never in Assessment lists or Lesson block
choosers.
