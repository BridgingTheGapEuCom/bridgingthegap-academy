# Course community domain

`community` owns Course-level discussions. Community follows durable Course ID,
not a CourseVersion or Draft, so new publications retain the same discussion
space. It stores only opaque authenticated user IDs for authorship.

A Community is enabled or disabled. Threads have a plain-text title and one
opening Post; the repository writes those two records in one PostgreSQL
transaction. Replies are flat Posts. Thread and Post state is `VISIBLE` or
`HIDDEN`; no physical deletion, role logic, moderation authorization, HTTP, or
frontend exists yet. Titles are limited to 240 characters and bodies to 20,000;
leading/trailing whitespace is normalized while internal line breaks remain.

## Participant API

Authenticated users may read and participate in an enabled community attached
to a Course that has published learner content. There is no enrollment model
yet, so this is intentionally the complete participation policy; it grants no
moderation capability and has no ADMIN bypass. All routes are scoped by durable
Course ID, use private `no-store` responses, and inherit session, Origin, and
CSRF protections.

Normal reads return only `VISIBLE` Threads and Posts. Missing, foreign-Course,
hidden, and disabled-community Threads are all represented as unavailable. A
disabled Community metadata response remains explicit while its thread list is
empty. Thread pages are ordered
by activity (`updatedAt DESC`, then ID); Post pages are chronological
(`createdAt ASC`, then ID). Creating a reply updates its Thread activity in the
same database statement. Participant DTOs carry only opaque author IDs and
plain text, never Identity account data or moderation state.
