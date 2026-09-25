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

Course and Dashboard placement, placement authorization, widget authoring,
plugin management UI, outbound networking, marketplace discovery, updates,
and forced termination of already-rendered frames remain deferred.

Security invariants:

- plugin code never executes in the Academy main JavaScript context;
- a plugin never receives Academy session credentials;
- a plugin receives only short-lived, explicitly granted runtime capabilities;
- manifest permissions do not grant runtime authority;
- trust and enablement are checked before every launch and token refresh.
