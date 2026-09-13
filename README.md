# Bridging the Gap LMS

An open-source, accessibility-first learning platform for structured, self-paced education. The HTTP API has login, logout, current-session, and a read-only administrator authorization check. No LMS business workflows or frontend authentication UI exist yet.

The architecture source of truth is [BTG_LMS_Architecture_Decision_Baseline_v5.docx](BTG_LMS_Architecture_Decision_Baseline_v5.docx), especially sections 32–33. [Module boundaries](docs/architecture-boundaries.md) documents the Go package owners and enforced dependency rules.

## Prerequisites

Install Go 1.25 or newer, Node.js 24, pnpm 10, Docker with Compose, and Make. Docker is needed for PostgreSQL and the Testcontainers test. The commands below assume a POSIX shell and ports 5432, 8080, and 5173 are free.

## Local development

From the repository root, prepare PostgreSQL and the frontend dependencies:

```sh
cp .env.example .env
pnpm install --frozen-lockfile
docker compose up -d db
set -a; . ./.env; set +a
go run ./cmd/btg-lms migrate
go run ./cmd/btg-lms doctor
```

Keep that terminal open and start the Go server:

```sh
go run ./cmd/btg-lms serve
```

In a second terminal, start Vite:

```sh
pnpm dev
```

Open `http://localhost:5173` for the development frontend. Vite proxies `/health` and `/api` to the Go server at `http://localhost:8080`. The Go server also serves the embedded production frontend at `http://localhost:8080`. `/health/live` checks the process; `/health/ready` checks PostgreSQL and schema compatibility. Prometheus metrics bind to `127.0.0.1:9090` by default. Server startup checks the schema but never applies migrations automatically.

## Initial administrator

After applying migrations, create an administrator from an interactive terminal:

```sh
go run ./cmd/btg-lms admin create --email admin@example.com
```

The command prompts twice for a non-echoed password. It rejects non-interactive standard input and does not accept a password argument, so passwords do not enter shell history or process listings. Passwords must be at least 12 characters and at most 1024 bytes. Credentials use Argon2id with 64 MiB memory, three iterations, one lane, a random 16-byte salt, and a 32-byte derived key; each encoded hash stores its own parameters for future upgrades.

## Internal session lifecycle

Identity can create and resolve server-side sessions without an HTTP transport. Each new session uses a fresh 32-byte random bearer token encoded as URL-safe base64; PostgreSQL stores only its SHA-256 digest. Sessions expire seven days after creation. Resolution checks expiry, revocation, and current user status, so suspending a user invalidates existing sessions. There is no idle timeout or automatic `last_seen_at` refresh yet. The Identity security observer signals explicit revocation and corrupt state. Authentication method and authorization roles are not stored in sessions at this stage.

The internal login operation authenticates a local password, then creates the session and appends a completed-login Audit event in one PostgreSQL transaction. It returns the bearer token only after commit. Logout accepts a trusted resolved session and transactionally revokes that session with a completed-logout Audit event; another session for the same user remains active. A retry with the same operation UUID reuses the matching logout audit fact; an already-revoked session creates no second completed-logout fact. Audit records include the acting user, session ID, authentication method for local login, and an operation UUID, but no password, bearer token, or digest. Failed password attempts continue to use the Identity security observer.

## HTTP session transport

`POST /api/auth/login` accepts JSON `email` and `password`, sets an opaque `btg_session` cookie, and returns `authenticated`, `user_id`, `expires_at`, and a separate `csrf_token`. `GET /api/auth/session` resolves the cookie against current server-side state and returns the same fields or a Problem Details `401`. Both responses use `Cache-Control: no-store`. `POST /api/auth/logout` revokes the resolved current session and clears the cookie; a valid session requires its token in `X-CSRF-Token`. An absent, expired, or revoked cookie can be cleared with `204 No Content` without a CSRF token because no server-side session is changed. Operational failures return Problem Details `500` without clearing the cookie.

The auth cookie is host-only, `HttpOnly`, `Path=/`, and `SameSite=Lax`, with expiry and Max-Age derived from the server-side session expiry. Production mode is the default and always sets `Secure`; serve it behind HTTPS. The `.env.example` explicitly selects `BTG_LMS_MODE=development` and binds HTTP to loopback, where the cookie may omit `Secure`. Client `X-Request-ID` headers are ignored: trusted middleware generates a fresh random request/operation ID.

CSRF uses a session-bound synchronizer token: each new session gets an independent 32-byte random token stored as sensitive Identity session state. The token is deliberately separate from the HttpOnly auth cookie and is never an authentication credential. The future Vue client should keep `csrf_token` in memory, fetch `/api/auth/session` after reload, send it as `X-CSRF-Token` on authenticated `POST`, `PUT`, `PATCH`, and `DELETE` requests, and discard it after logout; do not use localStorage or sessionStorage. A pre-M1.4b session gets a token once on its next current-session GET. Revocation and expiry make the token unusable. SameSite remains defense in depth, and CORS is not used as CSRF protection.

All mutating API requests, including login, require a single exact trusted `Origin`; missing, cross-origin, or lookalike origins receive Problem Details `403`. Login uses this pre-auth Origin check rather than an anonymous server-side session. `BTG_LMS_PUBLIC_ORIGIN` is the canonical HTTPS browser origin in production; development accepts only its configured loopback HTTP origin and the loopback Go address. Host and forwarded headers never define trust.

Login admission runs after Origin and bounded JSON validation, before password authentication. An in-memory token bucket allows six attempts per TCP peer initially and refills one attempt every 20 seconds; successful and unsuccessful attempts consume the same budget. It uses only the normalized `RemoteAddr` IP, never `X-Forwarded-For`, `X-Real-IP`, email, or account status. Buckets idle for 15 minutes are removed during periodic request-driven cleanup, and at most 4,096 sources are retained; when full, unseen sources fail closed instead of evicting active limits. A process-wide, fail-fast two-permit guard bounds concurrent login orchestration. Each login performs at most one Argon2 verification, including the dummy-hash path, so two verifications using the 64 MiB default need approximately 128 MiB of Argon2 working memory. Stored PHC parameters may specify up to 128 MiB per verification, making approximately 256 MiB the worst accepted two-login case; these figures do not include other process memory. Administrator bootstrap hashes in a separate CLI command, not alongside the serving login handler. These v1 constants are centralized in `internal/platform/login_limit.go`, not user-editable settings. Rejections return generic Problem Details `429`; per-source refill rejections include a whole-second `Retry-After`, while work saturation and full state do not claim a predictable wait. Prometheus counters use only coarse outcome/rejection labels, and no per-attempt Audit record is written.

These controls are per process, not shared across instances. Behind a reverse proxy, `RemoteAddr` identifies the proxy, so all clients may share its bucket. Do not trust forwarded IP headers without an explicit trusted-proxy deployment design. Full IPv6 addresses are separate buckets, which limits address-based protection against an attacker controlling a large prefix. There is no persistent account lockout.

`GET /api/admin/status` is a read-only proof of the central authorization boundary. Session middleware resolves the current user and constructs a minimal actor with user and session IDs; it never stores roles or bearer secrets in that actor. The Identity & Access authorization service evaluates the typed `instance.manage` capability against the current active global `ADMINISTRATOR` assignment on every request. Missing/invalid/suspended sessions receive Problem Details `401`; an authenticated actor without the capability receives `403`; role-storage failures receive a sanitized `500`. Granting or revoking the role changes the next request without replacing the session. The status handler never checks role rows itself. Protected mutations will run session resolution, CSRF, then capability authorization before their operation. A successful read-only status check creates no Audit event; coarse allowed/denied/unavailable authorization metrics have no user or resource-ID labels. Administrator status does not bypass immutable domain invariants, including future published CourseVersion rules.

## Build and generation

`make build` runs the Vite build and then produces the single `./btg-lms` executable. Run `./btg-lms version`, `./btg-lms doctor`, or `./btg-lms serve` after exporting `.env` as above. To refresh sqlc Go code and OpenAPI TypeScript types, run `make generate`. Identity owns its SQL queries and generated persistence package; the PostgreSQL connectivity query remains in Infrastructure.

`internal/web/dist` is committed because Go embeds it, including its hashed CSS and JavaScript assets. Run `pnpm build` after frontend changes; CI checks that generated assets and contract types match their sources.

## Tests

```sh
make check
make test-integration
pnpm exec playwright install chromium
make test-e2e
```

`make check` runs the architecture checker, Go vet, frontend build, Go tests, and Vitest. `make test-integration` starts a real PostgreSQL container through Testcontainers. `make test-e2e` runs Chromium and axe against the built frontend; run `pnpm build` first if you skipped `make check`.

## Container deployment

The Compose file starts PostgreSQL by default; the application is behind the `app` profile. Apply migrations explicitly before serving:

```sh
docker compose up -d db
docker compose run --rm --no-deps app migrate
export BTG_LMS_PUBLIC_ORIGIN=https://academy.example.com # replace with your actual browser origin
docker compose --profile app up -d app
curl http://localhost:8080/health/ready
```

When finished, stop the local deployment with `docker compose --profile app down`. The PostgreSQL volume remains unless you explicitly remove it.
