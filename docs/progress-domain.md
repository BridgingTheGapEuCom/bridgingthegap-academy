# Learner progress domain

`progress` owns private learner completion facts. A `CourseProgress` aggregate
is unique per durable Identity user and exact immutable Courses CourseVersion;
it contains a revision and the sorted set of completed published lesson stable
keys. It never references Drafts, Reviews, mutable Authoring lessons, or
Assessment authoring records.

Completion is monotonic. Marking a lesson complete inserts a single relational
fact and advances the revision only on the first insertion; replaying the same
completion returns the existing aggregate unchanged. A composite foreign key
ensures the lesson key belongs to the same published CourseVersion. Prerequisites
remain advisory and Assessment Attempts remain separate facts.

Course completion is a pure derived predicate: all version lesson keys must be
completed. An empty CourseVersion is deliberately incomplete. There is no HTTP
API, UI, enrollment side effect, version migration, reset, certificate, or
stored percentage in this foundation.
