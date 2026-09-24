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

## Authenticated authoring API

M8.1c exposes only private, authenticated translator routes:

- `POST` and `GET /api/courses/by-id/{courseId}/versions/{version}/translations`
- `GET` and `PATCH /api/translations/{translationId}`
- `POST /api/translations/{translationId}/publish`

The source route resolves an exact published CourseVersion. Creation accepts only
`targetLanguage`; the session supplies creator provenance. A duplicate workspace
returns `translation_already_exists` and never discloses it to a caller without
source-Course authority. Discovery is exact-version scoped and returns summaries,
not trees. Existing workspace routes load the stored Translation and authorize
against its stored source binding, so they use hidden 404 semantics for both
unknown and unauthorized IDs.

PATCH accepts a non-empty closed list of source-keyed text-field changes and a
required `expectedRevision`. It supports only modeled text overrides. `null`
clears an override back to untranslated and `""` records an intentional empty
translation. It rejects structural fields, assets, code bodies, Assessment
correctness, arbitrary JSON paths, and unknown source keys. Changes are applied
atomically; stale revisions return `translation_revision_conflict` without any
partial write.

Publication requires the authoritative current expected revision and a complete
workspace. `translation_incomplete` is returned without creating a snapshot.
Each successful publication stores an immutable snapshot; later edits cannot
alter it. Reads and publication validation always use the bound exact source
version, never Authoring or a latest-version lookup.

All authoring responses are `Cache-Control: no-store`. Session middleware
requires authentication and enforces the existing Origin/CSRF protections for
create, patch, and publication. Translation frontend and learner-facing
translation reads remain deferred.

## Translator authoring UI

The private translator UI uses `/authoring/courses/{courseId}/versions/{version}/translations` for exact-source discovery and creation, and `/authoring/translations/{translationId}` for a reload-safe workspace. It always displays the bound source SemVer and language, never a latest-version claim. Translators compare read-only source text with editable translation values, save explicit source-keyed changes manually, and receive the server-authoritative completeness result after a save.

An empty saved input is an intentional empty translation. The separate **Mark as untranslated** action sends `null`; it is the only UI action that clears an override. Save conflicts retain local text and offer a reload action. Publishing is available only when the server says the workspace is complete and requires a concise browser confirmation. The pages use grouped labels, semantic headings, native controls, textual status messages, and vertically stack naturally at narrow widths. Learner translation delivery, language selection, and source migration remain deferred.

## Immutable learner translated reads

Public translated reads resolve only the latest `TranslationPublication` for the
requested exact source CourseVersion and target language. They compose that
immutable snapshot with its exact immutable source CourseVersion; mutable
workspaces and Authoring state are never queried. Missing publications return
not found and no source-language fallback occurs. Intentional empty overrides
remain empty.

`GET /api/courses/by-id/{courseId}/versions/{version}/translations/{language}`
returns the learner-safe Course tree plus translation metadata and derived lag
metadata. Its `sourceLag` compares the translation’s fixed source SemVer with
the current latest source version only; it never changes rendered content. If
latest-version lookup is unavailable, the immutable translated course remains
readable and the optional latest/isLatest fields are absent. The combined
response uses the ordinary conservative public API caching policy because lag
can change. `GET .../languages` lists source first and published translation
languages for that exact source version only; it is discovery data and must not
be treated as immutable.

Translations preserve CourseVersion Assessment keys and grading identity,
source asset bindings, the durable Course Community, and Certificate identity.
Learner language selection and fallback remain deferred.

## Learner language selection

The exact CourseVersion reader accepts an explicit `lang` query parameter for a
published translation. The parameter is reload-safe and does not establish a
browser, account, cookie, or global language preference. The source reader is
canonical without `lang`; an unavailable requested translation shows an explicit
unavailable state with a source-version action, never a silent fallback.

When language discovery succeeds, the reader offers the source language and
only translations published for that exact source version. Discovery failure is
additive: the source Course remains usable. A lagging translation calmly states
its fixed translated source version and the newer source version, then offers a
learner-chosen route to that newer source version without carrying an unavailable
language query. Attempts, Community, and certificates retain their existing
CourseVersion/Course identities across presentation-language changes.
