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
