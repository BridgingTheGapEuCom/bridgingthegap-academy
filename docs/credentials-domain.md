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

Open Badges 3.0 is an interoperability target. Its achievement,
criteria, issuer, recipient, issuance, and revocation concepts can be mapped
from this internal model, but JSON-LD, Verifiable Credential proofs,
cryptosuites, signing, wallets, Badge Connect, and exports remain outside the
core domain. PDF rendering and automatic issuance triggers remain deferred.


## Learner and public reads

Authenticated learners issue a certificate only for an exact published CourseVersion. The server derives the learner from the session and delegates eligibility and idempotent replay to the issuance service; it accepts no certificate metadata from the request. Learners may read only certificates they own through the private API.

Public verification is an opaque certificate-ID lookup. It exposes frozen achievement, issuer, issued time, and `ACTIVE` or `REVOKED` status, but deliberately omits every recipient identity field. It is a registry-status statement, not a cryptographic verification claim. Verification uses `no-store` caching because revocation can change. PDF rendering remains deferred.

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
ID; the internal learner ID itself is never serialized. The unsigned `Map`
API still rejects revoked Certificates and emits no proof. The signing
pipeline uses `MapForSigning` with the durable status reference for both ACTIVE
and REVOKED Certificates. The unsigned adapter alone is not a portable signed
credential.

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
supplied `validFrom`; it has no proof or verification method. The publication
service signs and persists a complete snapshot before serving it. The list URL
stays stable while its signed representation refreshes after revocation.

## Signed Open Badges and revocation publication

Signed export is optional. It requires the stable HTTPS
`BTG_LMS_PUBLIC_ORIGIN`, the installation-stable
`BTG_LMS_OPEN_BADGES_SUBJECT_SECRET`, a frozen issuer ID matching
`<origin>/open-badges/issuer`, a stable
`BTG_LMS_OPEN_BADGES_KEY_ID` under that issuer URI, and a base64url-encoded
32-byte Ed25519 seed. Supply the seed through exactly one source:
`BTG_LMS_OPEN_BADGES_ED25519_SEED_B64URL`, or
`BTG_LMS_OPEN_BADGES_ED25519_SEED_FILE`. The file source is read once at
startup and must be a regular owner-only file (`0600` or stricter), owned by
the LMS service account. Startup rejects a missing companion setting, malformed
seed, mixed sources, non-HTTPS/loopback production origin, URL credentials,
inconsistent issuer/key IDs, and unsafe seed-file permissions. The feature is
disabled when no seed and key ID are configured: Certificate issuance and
registry verification continue normally, while signed-badge routes are absent.

Provision and back up the seed, the subject secret, and the PostgreSQL database
that retains public verification methods and signed status snapshots. The seed
is never generated at startup, persisted with credentials, returned by HTTP,
or printed by config diagnostics. Losing it must not lead to a replacement key
under the same issuer/key ID: old credentials remain mathematically verifiable
with their retained public key, but new signatures with that identity cannot be
created. Losing an active key requires an intentional new-key rollout; losing
only a historical private key does not prevent historical verification. Changing
the subject secret changes recipient pseudonyms, so do not rotate it without a
deliberate migration. `btg-lms doctor` reports only whether signed Open Badges
are disabled or whether the active public signing capability initializes; it
does not sign or publish anything.

`openbadges/signing` uses the TrustBloc VC Data Integrity implementation of
`eddsa-rdfc-2022`: JSON-LD safe-mode expansion, RDFC-1.0 canonicalization,
SHA-256, and Ed25519. Only pinned local VC 2.0 and OB3 3.0.3 contexts can be
loaded; signing and verification perform no remote context fetch. Proofs use
`DataIntegrityProof`, `assertionMethod`, an actual server signing time, and a
stable issuer verification-method URI. The public issuer controller document
publishes the Ed25519 Multikey and authorizes it for `assertionMethod` only.
The service verifies signatures against methods in that trusted controller
document, not keys supplied inside credentials. Historical method rows remain
published when a new key becomes active; operators must retain old public keys
for historical proof verification. Key compromise recovery is a separate
operational decision, not ordinary rotation.

`GET /api/public/open-badges/{certificateId}` serves the persisted signed
credential as `application/vc`; its stable credential identifier remains the
public verification URL and can return this machine-readable representation
with `Accept: application/vc`. Signed individual documents are immutable and
cacheable. `GET /open-badges/status/revocation/{listId}` serves only complete
signed Bitstring Status List credentials with `Cache-Control: no-store`; no
unsigned list is published. Issuer and exact CourseVersion Achievement
resources are public at their mapped URLs. The signed badge includes the
Certificate's stable list/index reference for both ACTIVE and REVOKED states.

`publication.Service.Revoke` is the signed-Open-Badges revocation workflow.
It establishes an ACTIVE baseline list, performs the one-way Certificate
transition, then signs and publishes the new list. It returns an explicit
`ErrRevocationPublicationFailed` if portable publication cannot be completed;
it never reports that outcome as success. In that narrow partial state the
Certificate is durably revoked, the old signed list remains stored, and the
public status route fails closed until a retry completes the fresh signed
snapshot. Retrying is idempotent: the Certificate, stable list entry, and list
revision are retained and only the missing current snapshot is rebuilt.

Certificate revocation increments the durable list lifecycle revision in the
same database transaction. Allocating a new status entry also increments it.
Publishing an unchanged snapshot and retrying an already-published revocation
does not. A snapshot is served only while its persisted revision matches the
current list revision; complete signed bytes are atomically replaced only after
they have been built, signed, and locally verified. Revocation never changes
the already-signed individual badge. This is a recovery-safe state machine,
rather than a cross-module transaction that holds external cryptographic work
inside the neutral Certificate repository.

Verification requires both a valid authorized issuer proof and the current
signed status-list proof/bit. A valid credential signature alone says nothing
about current revocation. Signed badges use long immutable caching;
exact-version Achievement resources use one day, issuer/controller documents
use five minutes, and revocable status lists use `no-store`. All public
responses omit learner identity, account data, role data, signing seeds, and
the subject secret.

The integration suite verifies ACTIVE and REVOKED documents with Digital
Bazaar's independent `@digitalbazaar/data-integrity` 2.0.0 and
`@digitalbazaar/eddsa-rdfc-2022-cryptosuite` 1.3.0 verifier. It uses a local,
allowlisted document loader and independently decodes the GZIP/multibase status
bit. It also proves that an old badge verifies after key rotation and that an
Ed25519 signature from a method absent from `assertionMethod` is rejected.
`pnpm test:openbadges-interop` verifies the checked-in deterministic ACTIVE
fixture with that independent verifier; the PostgreSQL integration test covers
the corresponding ACTIVE-to-REVOKED snapshot transition. This is
interoperability evidence, not a 1EdTech certification claim. Formal
1EdTech certification, wallet integration, and external certification workflow
remain outside this release.
