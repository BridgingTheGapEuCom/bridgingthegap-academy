# Credentials domain

`credentials` owns durable awarded certificates. A Certificate is an opaque
historical achievement for one authenticated learner and one exact immutable
Courses `CourseVersion`. It stores both the durable internal CourseVersion ID
and its Course ID, while freezing the Course title, SemVer, language, and a
course-completion criteria statement at issuance. A later CourseVersion cannot
change the meaning of an earlier certificate.

The recipient is only the existing opaque Identity user ID. Certificates do
not copy email, login, roles, authentication data, or mutable Authoring
provenance. The issuer is a small frozen `{id, name}` value supplied from
application configuration at issuance, rather than a hard-coded product name
or an issuer-management domain.

Certificates are `ACTIVE` or `REVOKED`. Issuance facts are immutable. Revoking
is a one-way, non-destructive transition that records a server-owned time; the
record is never normally deleted. PostgreSQL enforces one Certificate per
learner and exact CourseVersion, so a retry converges to the already-issued
record instead of minting a duplicate.

This foundation contains no automatic issuance or completion policy. A later
server-authoritative eligibility service must derive an exact published
CourseVersion and create the frozen snapshot. Assessment Attempts remain
separate facts and a single Attempt does not itself award a certificate.

Open Badges 3.0 is a future interoperability target. Its achievement,
criteria, issuer, recipient, issuance, and revocation concepts can be mapped
from this internal model, but JSON-LD, Verifiable Credential proofs,
cryptosuites, signing, wallets, Badge Connect, and exports remain outside the
core domain. PDF rendering, public verification, HTTP APIs, and learner UI are
also deferred.
