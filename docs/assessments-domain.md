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

Canonical `KNOWLEDGE_CHECK.assessmentKey` already accepts the lowercase UUID
format used by Assessment IDs. This milestone does not resolve that reference:
publication continues to return `unresolved_assessment_reference`, and there
are no learner attempts, grading, or publication bindings yet.

Assessment deletion, cross-Draft reuse, free-form grading, and external
assessment engines are deliberately unsupported. Later publication must freeze
an immutable deterministic representation rather than make a CourseVersion
depend on mutable Authoring state.
