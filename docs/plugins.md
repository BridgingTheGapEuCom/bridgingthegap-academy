# Plugin trust and registry foundation

The Plugins module establishes static package identity, integrity, trust, and
local installation state. It does not execute plugin resources. Plugin code is
not loaded into the Go process, imported as JavaScript, run as WebAssembly, or
contacted over the network during package validation or registration.

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

## Runtime boundary and future work

The long-term invariant is that a plugin receives only explicitly granted
runtime capabilities. It must never inherit server filesystem, database,
environment, user tokens, or unrestricted network authority. A later widget
runtime will map validated course widget references through a controlled block
and sandbox boundary; this foundation does not modify canonical LessonContent.
Dashboard rendering, sandboxing, marketplace discovery, downloads, updates,
signing services, and runtime unloading after trust changes remain deferred.
