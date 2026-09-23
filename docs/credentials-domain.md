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

Certificate issuance is a transport-independent application service. Its v1
rule is explicit: the learner must have durable `progress.CourseProgress` facts
for **every published Lesson** in the exact immutable CourseVersion. A version
with no Lessons is not issuable because it has no completion signal. Progress
belongs to the exact CourseVersion, so facts for another learner or Version do
not qualify. Assessment Attempts remain separate facts: submitted Attempts,
scores, Community activity, reader visits, and frontend state neither grant nor
block this v1 rule.

`BTG_LMS_CERTIFICATE_ISSUER_ID` and `BTG_LMS_CERTIFICATE_ISSUER_NAME` are
required server configuration and are validated at startup. They are frozen
into each Certificate. The criteria text states the all-published-Lessons rule.
Issuance first returns any existing Certificate for the learner and exact
CourseVersion, including a REVOKED Certificate, before evaluating current
eligibility. This makes retries and concurrent issuance converge and never
silently replaces or restores historical credentials.

Open Badges 3.0 is a future interoperability target. Its achievement,
criteria, issuer, recipient, issuance, and revocation concepts can be mapped
from this internal model, but JSON-LD, Verifiable Credential proofs,
cryptosuites, signing, wallets, Badge Connect, and exports remain outside the
core domain. PDF rendering, public verification, HTTP APIs, automatic
issuance triggers, and learner UI are also deferred.


## Learner and public reads

Authenticated learners issue a certificate only for an exact published CourseVersion. The server derives the learner from the session and delegates eligibility and idempotent replay to the issuance service; it accepts no certificate metadata from the request. Learners may read only certificates they own through the private API.

Public verification is an opaque certificate-ID lookup. It exposes frozen achievement, issuer, issued time, and `ACTIVE` or `REVOKED` status, but deliberately omits every recipient identity field. It is a registry-status statement, not a cryptographic verification or Open Badges claim. Verification uses conservative `no-store` caching because revocation can change. PDF rendering, Open Badges/VC serialization and signing remain deferred.

## Open Badges 3.0 unsigned adapter

`internal/modules/credentials/openbadges` is the only Open Badges/VC-aware
boundary. It maps an ACTIVE Certificate snapshot to an unsigned VC 2.0/Open
Badges 3.0 JSON-LD document with the VC 2.0 context followed by the Open Badges
3.0.3 context, and `VerifiableCredential` plus `OpenBadgeCredential` types.
The adapter never reads Courses, Authoring, Identity, or current configuration
for a Certificate's frozen achievement and issuer fields.

A configured HTTPS public base URL supplies stable credential and achievement
identifiers. The recipient is an issuer-scoped HMAC-SHA-256 pseudonymous URI
created from a deployment-held 32-byte-or-longer salt and the internal learner
ID; the internal learner ID itself is never serialized. Revoked Certificates
are deliberately rejected for portable export until a truthful standard
credential-status service exists. No `proof` is emitted. This is an unsigned
pre-signing representation, not an Open Badges-conformant signed credential or
cryptographic verification result.

## Open Badges publication prerequisites

The deployment-held `BTG_LMS_OPEN_BADGES_SUBJECT_SECRET` is validated at a
minimum of 32 bytes when configured. It is never returned or logged. It is an
installation-stable credential-identity secret: rotating it would change the
HMAC-derived recipient URI on a reserialization, so rotation is a deliberate
future migration, not a routine configuration change.

The unsigned adapter derives the credential identifier from the Academy public
verification route (`/verify/certificates/{certificateId}`), and creates an
exact CourseVersion Achievement identifier at
`/achievements/course-versions/{courseVersionId}`. Frozen issuer claims remain
from the Certificate; current configuration does not rewrite historical
claims.

`open_badges.status_list` and `open_badges.status_list_entry` reserve stable
W3C Bitstring Status List v1.0 `revocation` positions. Lists have the W3C
minimum capacity of 131,072 entries. New allocations use cryptographically
secure random indices, with database uniqueness, bounded collision retries, and
a free-slot fallback near capacity. Existing allocated indices stay unchanged.
`next_index` tracks the number of reserved slots, not the allocated index; a
new list is created when the current list reaches capacity. The entry is
idempotent per Certificate and does not store lifecycle truth: `credentials.certificate.status` remains authoritative. ACTIVE and
REVOKED Certificates retain the same reserved entry, so a later signed status
list can change the bit without re-signing the credential.

The prepared `BitstringStatusListEntry` uses a decimal-string index and an
entry ID distinct from the list URL. It omits optional `statusSize`, using the
standard one-bit default; it is not attached to exported unsigned badges.
The internal builder reads allocation and Certificate lifecycle from one
consistent database snapshot. It creates a full-size bitstring (index zero is
the most significant bit of the first byte), GZIP-compresses it, then encodes
it as multibase base64url without padding. A typed, unsigned
`BitstringStatusListCredential` snapshot can be constructed with an explicitly
supplied `validFrom`; it has no proof or verification method. Publishing a
status-list VC requires constructing, signing, and publishing a complete
snapshot atomically. The list URL stays stable while its signed representation
can be refreshed after revocation. No status-list VC is published yet.
Portable revocation verification remains unavailable until the status-list
credential is cryptographically secured and published.
