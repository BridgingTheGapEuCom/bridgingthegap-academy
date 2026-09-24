# Translations domain

`translations.CourseTranslation` is a mutable, derived workspace. It is not a
CourseVersion and never changes a source CourseVersion. Every workspace binds
once to one published immutable source `(Course ID, CourseVersion ID, SemVer,
source language)` and one distinct normalized target language. PostgreSQL
enforces one workspace per exact source CourseVersion and target language.

The workspace tree mirrors the source Module, Lesson, LessonContent-block, and
Assessment structure by stable source key and order. It contains nullable
translated text overrides only: `nil` means untranslated; a present empty string
means an intentional empty translation. It never copies source prose as if it
were translated. Source text remains available through the immutable source
version when a future translator UI needs to compare it.

Course/module/lesson titles and descriptions, objectives, supported textual
block fields, accessibility text (such as image alt/caption and media title or
transcript), and learner-facing Assessment prompts/options/items are eligible
for overrides. IDs, stable keys, ordering, duration, prerequisites, asset
bindings, code bodies, URLs, CourseVersion identity/SemVer, licenses, and
publication provenance are source-owned. Assessment correctness, question type,
option/item identity, and matching pairs are never represented in the
translation tree, so a translation cannot change grading semantics.

The draft has a monotonic revision and uses compare-and-swap updates. A publish
creates a distinct immutable `TranslationPublication` snapshot; later draft
edits do not alter earlier publications. Repeated publication of a workspace is
allowed and the latest publication can be resolved only for the same exact
source CourseVersion and target language. Translation publications do not have
an independent Course SemVer.

Publishing source `1.1.0` never retargets, copies into, or creates a translation
for a source `1.0.0` workspace. A later read can derive source lag by comparing
the persisted exact source binding with a Course's latest version; no mutable
`isStale` flag is stored.

The module reads only immutable Courses data and owns only `translations.*`
tables. It has no HTTP API, authoring/learner UI, language fallback, translation
review/membership workflow, machine/AI translation, notification, diff,
import/export, or translated certificate/Open Badge metadata. Those require
later product slices.

## Authorization and translator workspace reads

The application boundary exposes `translation.read`, `translation.create`,
`translation.edit`, and `translation.publish`. For v1 all four are granted to a
user recorded in published source-Course attribution as an `AUTHOR` or
`MAINTAINER`. The policy is scoped to the Translation's durable source Course;
the creator ID is private provenance only and does not confer permanent access.
Global `ADMINISTRATOR` status has no implicit Translation bypass. Future
translator membership can extend this capability resolver without changing the
Translation aggregate or persistence schema. Unauthorized callers receive a
domain authorization denial suitable for a later HTTP boundary to map to a
hidden resource response.

`TranslationWorkspaceView` is the private, translator-facing query model. It
combines the persisted workspace with its exact immutable source CourseVersion,
never a latest CourseVersion or Authoring Draft. Every supported translatable
field is a source/translated pair plus `UNTRANSLATED` or `TRANSLATED` state.
`nil` is untranslated, while a present empty string is intentionally translated
and counts as translated. The view provides deterministic total, translated,
untranslated, and complete counts for the whole workspace and useful structure
levels. A zero-field structure is complete with all counts zero.

Block views retain type, source key, order, and read-only context such as asset
keys, code payload, table cells, or a knowledge-check Assessment reference.
Only fields modeled by the foundation participate in completeness. Assessment
views include learner-facing prompt, option, and matching labels with stable
keys/order, but deliberately omit correct-option keys, matching pairs, and
other grading internals. If the stored translation tree cannot exactly reconcile
with its bound source keys and structure, workspace reading fails with a
controlled source-integrity error rather than guessing or migrating data.
