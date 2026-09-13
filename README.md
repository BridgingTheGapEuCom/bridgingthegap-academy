# Bridging the Gap LMS

An open-source, accessibility-first learning platform for structured, self-paced education. M1.1 adds Identity persistence only; there are no authentication flows, LMS business workflows, or public `/api/v1` operations.

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

Open `http://localhost:5173` for the development frontend. Vite proxies `/health` to the Go server at `http://localhost:8080`. The Go server also serves the embedded production frontend at `http://localhost:8080`. `/health/live` checks the process; `/health/ready` checks PostgreSQL and schema compatibility. Prometheus metrics bind to `127.0.0.1:9090` by default. Server startup checks the schema but never applies migrations automatically.

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
docker compose --profile app up -d app
curl http://localhost:8080/health/ready
```

When finished, stop the local deployment with `docker compose --profile app down`. The PostgreSQL volume remains unless you explicitly remove it.
