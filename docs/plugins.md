# Plugin trust and registry foundation

The Plugins module establishes package identity, integrity, trust, local
installation state, and an isolated browser runtime. Plugin code is never
loaded into the Go process or Academy's main JavaScript context and is never
executed during validation or registration.

## Package and manifest v1

A plugin is an immutable release identified by a reverse-domain plugin ID and
a three-part SemVer, for example `com.example.academy.timeline` at `1.4.2`.
The same ID and version may be registered again only when its artifact digest
is identical. Changed content requires a new version.

The ZIP layout is deliberately closed:

```text
manifest.json
checksums.json
signature.json       # optional
resources/...
```

`manifest.json` has `format: bridging-the-gap-plugin` and `formatVersion: 1`.
It contains display metadata, the publisher display name, declared plugin
types, entrypoints, permissions, and a sorted resource inventory containing
path, byte size, and SHA-256 digest. The only v1 plugin types are
`COURSE_WIDGET` and `DASHBOARD_WIDGET`. Entrypoint IDs are stable within a
plugin, so a widget type is identified by plugin ID plus widget ID. The only v1
permission is `NONE`; network, database, filesystem, process, environment, and
session access are not implied or expressible.

Parsing is strict. Unknown or duplicate JSON fields, trailing JSON, undeclared
files, missing files, unsafe or duplicate ZIP paths, symlinks/special entries,
checksum differences, unsupported types, and unsupported format versions are
rejected. Compressed bytes, expanded bytes, entry bytes, entry count, and
compression ratio are bounded while reading. Resources are treated as opaque
bytes and are never executed. Validation produces an opaque
`ValidatedPluginPackage`.

## Integrity and signatures

The artifact digest is SHA-256 over the canonical v1 manifest JSON and its
ordered resource inventory. The Ed25519 signing payload is exactly:

```text
btg-plugin-signature-v1
<lowercase artifact SHA-256>
<canonical manifest JSON>
```

The three lines are joined with a single LF byte. There is no trailing LF
after the canonical manifest JSON.

The manifest inventory binds every resource path, size, and SHA-256 digest, so
changing metadata, an entrypoint, permissions, or any resource invalidates the
artifact identity and signature. ZIP timestamps and ZIP entry order are not
part of the signed identity.

`signature.json` names the `Ed25519` algorithm, a stable key ID, and a raw
URL-safe base64 signature. Verification keys are installation-owned records:
key ID, public key, purpose, allowed plugin identities for owned keys, and an
enabled flag. A key ID cannot be rebound to different key material. Historical
public keys can remain registered after rotation. Private signing keys are not
stored by the plugin registry.

## Validity and trust

Package controls metadata; installation controls trust. A manifest cannot
self-declare trust, and a trusted-looking ID prefix has no effect.

- `BTG_OWNED` requires a valid signature from an enabled key registered for
  owned signing and an explicit plugin-ID allowlist match.
- `BTG_APPROVED` requires a valid signature from an enabled BTG approval key
  and an active approval for the exact plugin ID, version, artifact digest, and
  authority key.
- `UNKNOWN` is a valid unsigned package, a package signed by an unrecognized
  key, a recognized signature outside an owned allowlist, or a new release
  without its own approval.

Cryptographic validity is not administrative approval. Approval of `1.0.0`
does not approve `1.1.0`. A claimed signature that can be checked against a
recognized key but fails verification is invalid and is rejected; it does not
degrade to `UNKNOWN`.

Approvals are append-only release records with an optional revocation time.
BTG approvals name their approval authority key. A separate local approval can
permit an unknown release under an installation policy without changing its
trust classification. Revocation and key disablement are retained rather than
deleting history.

## Local registry and enablement

The local registry persists an opaque installation ID, immutable release
identity, validated manifest, signing evidence, trust observed at registration,
installation time, and `INSTALLED` or `DISABLED` state. Exact registration is
idempotent. Reusing ID and version for another digest is a conflict enforced by
the database uniqueness boundary and repository comparison.

Trust is re-evaluated from current keys and active approvals whenever a release
is read for policy or enabled. The stored registration classification is
history, not permanent authority. Owned and approved releases can be enabled.
For unknown releases, `Policy.AllowUnknown` must be enabled and the configured
manual-approval requirement must be satisfied. Secure zero-value policy denies
unknown enablement. Invalid packages are never installable.

Future administrative operations require the installation-scoped
`plugins.manage` capability. No HTTP management API is part of this slice.
Individual widget placement authorization is also deferred.

## Widget runtime boundary

Widget JavaScript runs only in an iframe on the separately configured
`BTG_LMS_PLUGIN_RUNTIME_ORIGIN`. Production requires HTTPS and a host different
from `BTG_LMS_PUBLIC_ORIGIN`; development also requires a distinct host (for
example `plugins.localhost`). There is no same-origin production fallback. The
Academy session cookie is host-only, is not sent to the runtime host, and is
never accepted by runtime APIs.

The iframe sandbox is exactly `allow-scripts allow-same-origin`. Combining
those flags is safe here because the runtime origin is required to be distinct
from the Academy origin. It lets both sides enforce an exact `postMessage`
origin instead of using `targetOrigin: "*"`. Top navigation, popups, forms,
modals, downloads, and opener control remain unavailable.

Runtime pages use a restrictive CSP beginning with `default-src 'none'`.
Scripts, styles, images, and media are limited to the plugin runtime origin;
forms, objects, frames, workers, fonts, manifests, and base URLs are disabled.
`frame-ancestors` names only the Academy host. There is no CDN or arbitrary
external network access, and `connect-src 'none'` prevents plugin code from
calling even the runtime origin. The context and refresh APIs establish the
host-mediated capability boundary for future placement work; widget code does
not call them directly in v1.

Validated resource bytes and their path, size, and SHA-256 identity are stored
in the plugin-owned registry transaction. Resource URLs include plugin ID,
version, and artifact digest. Lookups use exact inventory identity rather than
filesystem path concatenation, and bytes are rechecked against their digest
before launch or serving. Declared immutable resources may remain cacheable
after disablement; possession of bytes grants no runtime authority.

## Runtime credentials and protocol

Every launch receives a fresh opaque UUID runtime instance and a five-minute
Ed25519 capability token. Claims bind issuer `btg-academy`, the dedicated
`btg-widget-runtime` audience,
runtime instance, plugin ID and version, artifact digest, widget ID and type,
explicit grants, issue time, and expiry. The signing seed and key ID come from
the widget-runtime configuration and are separate from sessions, Open Badges,
and plugin package/approval keys. Tokens are held in memory and delivered by
the host handshake, never in a URL or cookie.

The initial grants are only `widget.runtime.bootstrap` and
`widget.runtime.context.read`. Manifest permission is a request declaration;
it never becomes a runtime grant automatically. Runtime endpoints accept one
Bearer token and never fall back to the Academy session cookie.

Host and iframe exchange strict envelopes with protocol
`btg-widget-runtime`, version `1`, message type, runtime instance ID, and
payload. Startup is `WIDGET_READY`, `RUNTIME_INIT`, then
`RUNTIME_INITIALIZED`. Both sides check the exact other origin; the host also
checks `event.source` against its iframe. Unknown messages, sources, origins,
instances, and protocol versions are ignored. `BTG_RUNTIME_DISABLED` is an
optional advisory shutdown message and is not the security boundary.

Every launch and refresh re-evaluates enabled state, current keys, approvals,
local unknown-plugin policy, widget declaration, and entrypoint integrity.
Disablement, approval revocation, or key disablement therefore blocks new
launches and refresh immediately. An already issued token remains usable until
its short expiry; after that runtime API access ends, and a reload cannot
relaunch. This avoids a durable per-render revocation table.

## Course widget placement

Courses use one canonical `PLUGIN_WIDGET` LessonContent block. Its existing
block key is the placement key and the immutable payload pins plugin ID,
SemVer, artifact digest, Course-widget entrypoint ID, and bounded JSON
configuration. Configuration is data only: it is never evaluated as HTML or
JavaScript. Course authors use their existing Lesson-content capability and
revision CAS; `plugins.manage` remains installation administration only.

Only enabled releases that currently satisfy trust policy appear in Course
authoring discovery. Draft replacement, Review submission, and publication
all validate the exact pinned release. A disabled or revoked release blocks a
new publication instead of stripping the block. Review and published Course
snapshots retain their exact release coordinates forever; an upgrade never
rewrites an existing CourseVersion.

Learners receive a safe block projection and launch by Course ID, exact
version, Lesson key, and placement key. The server resolves the immutable
CourseVersion and derives the release and configuration itself. A Course
launch receives the additional narrow `widget.course.context.read` grant.
Its server-authored placement context contains only Course/version IDs,
Lesson/placement keys, presentation language, and configuration—never learner
identity, progress, assessment answers, Community data, or session material.
The launch descriptor conveys that context in the existing handshake because
the runtime CSP deliberately has `connect-src 'none'`; the protected context
endpoint remains available to a future host-mediated boundary.

If a pinned release is later disabled or loses trust, the CourseVersion remains
unchanged and the rest of the Course reads normally. The widget renders a calm
unavailable state and cannot obtain a fresh launch or refreshed token.
Portability preserves widget placement metadata but never bundles plugin code:
an import succeeds only when the target installation already has the exact
enabled, trusted ID/version/digest/widget release. It never downloads,
substitutes, or removes a widget.

Dashboard frontend rendering, plugin management UI, marketplace discovery,
updates, outbound networking, and forced termination of already-rendered
frames remain deferred.

## Plugin management read API

`GET /api/plugins` and `GET /api/plugins/{pluginId}/{version}` are
installation-management reads protected by `plugins.manage`. They use
authenticated Academy sessions and `Cache-Control: no-store`; Dashboard
placement authority, Course authoring authority, and widget-runtime tokens do
not authorize them. The list is intentionally unpaginated because the local
installed-release registry is expected to remain small. It is deterministically
ordered by plugin ID ascending and SemVer descending.

The read projection exposes immutable release coordinates, safe manifest display
metadata, widget entrypoint IDs/types/names, installed time, current derived
trust, approval state, stored enablement, and current execution-policy
eligibility. Those values deliberately remain separate: an approved release is
not automatically enabled, an enabled release may no longer be execution
eligible after trust changes, and neither state says whether a release is placed
or running anywhere.

Approval is projected as `NONE`, `ACTIVE`, or `REVOKED` for the exact
ID/version/digest release. It neither transfers to a later version nor exposes
approval history or authority-key material. `UNKNOWN` is a valid current trust
classification for a structurally valid installed package; it never means that
a failed recognized signature was accepted. Releases are strictly validated at
registration, but the registry does not retain a separate full historical
archive-validation log. A currently recognized signature that no longer
verifies is reported as `SIGNATURE_INVALID` with no `currentTrust`, rather than
being relabeled `UNKNOWN`.

The management projection excludes private key material, raw signatures,
resource bytes or storage paths, internal installation IDs, runtime bearer
tokens, and Academy session data. It executes no plugin code. Approval,
revocation, enablement, key administration, and a plugin-management UI remain
future M10.3 work.

`POST /api/plugins` registers one `application/zip` plugin package and also
requires `plugins.manage`, a trusted Origin, and the session-bound CSRF token.
The request is bounded by the canonical package reader's compressed archive
limit; that reader performs the hostile-ZIP, strict JSON, manifest, inventory,
checksum, and signature validation before `RegistryService` persists anything.
Package installation is data processing only: it does not import a module, run
a script, invoke a build hook, or contact a network service.

The archive itself supplies the immutable ID/version/digest identity. An exact
same-digest replay is idempotent and returns the existing management projection;
the same ID/version with different content/digest conflicts without replacing
the registered release or resources. Release and resource rows are committed in
one PostgreSQL transaction, so a failed resource validation or persistence step
rolls the whole registration back. Trust is then derived from local keys and
approval records. Registration neither enables a release nor creates an
administrative approval, places a widget, or issues runtime credentials. A
recognized invalid signature fails registration; it is never treated as an
`UNKNOWN` package.

## Plugin trust lifecycle management

All lifecycle routes use an authenticated Academy session, `plugins.manage`, a
trusted Origin, and the session-bound CSRF token. Exact releases support
`POST .../approval`, `DELETE .../approval`, `POST .../enable`, and
`POST .../disable`. Mutation responses reuse the management release projection,
so current trust, approval history state, stored enablement, and execution
eligibility remain visibly separate.

Administrative approval applies only to the installed ID/version/digest and is
available only when that release is already signed by an active
`BTG_APPROVAL_SIGNING` key. The administrator action activates that existing
cryptographic approval chain; it cannot turn an unsigned or independently
signed package into `BTG_APPROVED`. Repeating an active approval is idempotent.
Revocation marks the record inactive without deleting history, is idempotent
once matching revoked history exists, and immediately changes derived trust and
runtime launch/refresh eligibility. It never transfers to another release or
plugin.

Enablement re-evaluates current trust and policy. `UNKNOWN` remains denied by
the strict instance policy, even to a plugin administrator. Disablement retains
the immutable release, resources, CourseVersion blocks, and Dashboard
placements. Trust or key revocation does not silently rewrite a stored enabled
flag; the management projection instead reports that stored state separately
from current execution eligibility. New launches and refreshes fail while
existing runtime tokens retain only their normal short lifetime.

Verification-key administration is exposed through `GET/POST /api/plugins/keys`
and the key `.../enable` and `.../disable` actions. The only supported purposes
are the existing `BTG_OWNED_SIGNING` and `BTG_APPROVAL_SIGNING` values. Owned
keys require an explicit plugin-ID allow-list; approval keys require an empty
allow-list. The create request accepts exactly 32 Ed25519 public-key bytes as
unpadded base64url. Responses expose a SHA-256 public-key fingerprint rather
than key bytes. Expanded 64-byte private keys are rejected. A raw 32-byte
Ed25519 seed is mathematically indistinguishable from arbitrary 32-byte public
material, so operators must supply the public key; this API never stores or
returns private signing material. Key deletion, remote discovery, automatic
rotation, and private-key signing services are not supported.

The existing audit store is intentionally constrained to identity/session
events and UUID identity resources. Plugin lifecycle mutations therefore do not
write a separate non-transactional audit stream in this slice; extending the
shared audit schema and transaction boundary is deferred rather than inventing
an isolated plugin log.

## Dashboard placement management

The Academy has no Dashboard aggregate, user preference store, or per-user
Dashboard model. Dashboard widget placement is therefore installation-wide
configuration, not user-owned state. Each placement pins an exact plugin ID,
version, artifact digest, and Dashboard entrypoint; plugin upgrades never
rewrite it. Placement configuration is bounded JSON data only.

Placements form one deterministic vertical ordered list. Creation appends to
the list. Move operations submit the full observed placement-revision map and
are committed in one PostgreSQL transaction: rows are locked, every revision
is checked, temporary positions avoid uniqueness collisions, and contiguous
positions plus new revisions are written atomically. Configuration, enabled
state, deletion, and moves all use CAS and report a conflict instead of
overwriting a newer layout.

`dashboard.widgets.manage` controls this installation configuration. It is
separate from `plugins.manage`, which concerns plugin registry administration.
Discovery returns only enabled, currently eligible `DASHBOARD_WIDGET`
entrypoints and does not expose signing or approval internals. A placement is
not rewritten or deleted if its release later becomes unavailable.
Placements are installation-local and never appear in Course portability
packages.

Dashboard runtime launch uses the same generic sandbox, runtime origin,
Ed25519 token issuer, and refresh endpoint as Course widgets. The Academy-host
request supplies only the opaque placement ID. The server loads that placement
and derives its exact plugin ID, version, artifact digest, Dashboard entrypoint,
and configuration; clients cannot select or replace those values. Launches get
only the additional `widget.dashboard.context.read` grant. The Dashboard
context contains exactly the placement ID and bounded configuration, with no
user identity, session, role, Course, progress, Assessment, certificate, or
Community data.

Like the existing Course runtime, Dashboard configuration is snapshotted for
one runtime UUID at launch. An edit affects subsequent launches; it does not
silently change a running widget. Refresh still reloads the placement and
rechecks that it exists, is enabled, retains the exact pinned release/widget,
and that the release remains enabled, trusted, approved where required, and
integrity-valid. Disabling or deleting the placement, or disabling/revoking its
plugin, blocks new launches and refresh while an already-issued token and its
snapshot retain only their normal short lifetime. Placement UUIDs are never
reused, so a deleted placement cannot be rebound to another runtime context.

## Dashboard frontend

The authenticated Dashboard route loads the installation-wide ordered placement
list from the server and treats its response as authoritative after every
mutation. It renders each enabled placement with the same generic
`WidgetRuntimeFrame` used by Course widgets. A frame launch sends only the
opaque placement ID; a failing or unavailable frame is contained in that
widget's own region and never prevents the rest of the Dashboard from working.

Users with the server-confirmed `dashboard.widgets.manage` capability can add
from Dashboard-widget discovery, edit bounded JSON configuration, move a
widget up or down, enable or disable a placement, and remove a placement. The
controls use native buttons and forms; reordering is keyboard-accessible and
sends the complete current revision map. Conflicts reload the authoritative
list rather than attempting a browser-side merge. Configuration or enabled
state changes remount a fresh runtime, so a new launch receives its own
configuration snapshot. There is no per-user layout, drag-and-drop, grid, or
browser persistence in v1.

Security invariants:

- plugin code never executes in the Academy main JavaScript context;
- a plugin never receives Academy session credentials;
- a plugin receives only short-lived, explicitly granted runtime capabilities;
- manifest permissions do not grant runtime authority;
- trust and enablement are checked before every launch and token refresh.

## Plugin management frontend

`/admin/plugins` is the administrator-facing surface for the existing
`plugins.manage` APIs. It lists installed releases in the server's deterministic
order and keeps registration validation, derived trust, approval state, stored
enablement, and current execution eligibility as separate fields. In particular,
`UNKNOWN` means a valid installed release without current BTG ownership or
approval; it is not an invalid package. A recognized signature failure is shown
as `SIGNATURE_INVALID`, never as `UNKNOWN`.

Administrators select a ZIP package with the native file input and explicitly
install it as `application/zip`. The browser does not parse package identity or
trust evidence. The server's canonical result determines the release shown, and
the UI distinguishes a new `201` registration from an idempotent `200` replay.
No package bytes, runtime credentials, or management state are stored in browser
storage.

The page also exposes exact-release approval/revocation and enable/disable
controls. These actions reload authoritative release state; revocation or
disablement does not delete CourseVersions or Dashboard placements, but can make
future widget launches unavailable. Verification-key administration is limited
to adding and enabling/disabling public verification keys. Owned signing keys
need an explicit plugin allow-list, approval keys do not. The UI warns that raw
32-byte Ed25519 seed bytes cannot be distinguished from public bytes by length,
so administrators must never paste private seed or signing material.

The interface uses responsive cards, semantic regions, labelled native controls,
live status and error announcements, keyboard-accessible lifecycle actions, and
wrapping identifiers/fingerprints for narrow screens. It has no uninstall,
marketplace, automatic update, key deletion, private-key management, or browser
signing capability.

## M10 v1 lifecycle closure

The v1 lifecycle keeps each security decision explicit. Registration records a
strictly validated immutable package. Current trust is recomputed from its
signature, active local verification keys, owned-signer allow-list, and exact
release approval history. Stored enablement is an independent administrator
choice. Execution is permitted only when current trust/policy and stored
enablement both allow the exact release; Course and Dashboard placements remain
separate immutable or installation-local references.

| Package and installation state | Current trust | Stored enablement | Runtime result |
| --- | --- | --- | --- |
| Valid owned-signed release with active allowed owned key | `BTG_OWNED` | disabled | installed, not launchable |
| Valid approval-signed release with active exact-release approval and key | `BTG_APPROVED` | enabled | launchable |
| Valid unsigned, unrecognized, unapproved, or no-longer-recognized release | `UNKNOWN` | either | denied by the strict official policy |
| Recognized signature fails verification or content integrity fails | no valid trust classification | either | invalid/ineligible; never downgraded to `UNKNOWN` |
| Previously approved release after approval or authority revocation | normally `UNKNOWN` | may remain enabled | new launch and refresh denied; historical approval and placements retained |

Approval, key disablement, and release disablement never rewrite a published
CourseVersion or a Dashboard placement and never substitute a newer plugin
version. Existing runtime tokens retain only their original short expiry;
lifecycle mutations deny new launches and refreshes without adding a global token
revocation system. Course tokens carry only Course context authority and
Dashboard tokens carry only Dashboard context authority. Widget bearer tokens
cannot authenticate management routes, and Academy sessions cannot substitute
for runtime bearer credentials.

The v1 package, trust, placement, runtime, and management implementation is now
closed for M10. Deliberately deferred work includes plugin uninstall or release
replacement, remote marketplaces, automatic updates or widget migrations,
trusted-key deletion or automatic rotation, private signing-key management,
browser-side signing, backend/authentication/database plugins, arbitrary network
or filesystem/database access, cross-widget communication, per-user persistent
widget state, complex Dashboard grids, and treating Course-portability packages
as plugin trust/signature containers.

Plugin lifecycle audit remains future cross-domain audit architecture work. The
current `audit.events` model is designed for identity/session UUID resources;
M10 does not add a plugin-specific, non-transactional audit log that could drift
from the lifecycle transaction.
