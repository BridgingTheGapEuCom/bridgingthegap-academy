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
