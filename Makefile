.PHONY: install generate frontend-embed frontend-build build test test-integration test-e2e lint lint-go check clean dev db-up migrate doctor

GOLANGCI_LINT_VERSION := $(shell cat tools/golangci-lint-version)
GO_TOOLCHAIN_VERSION := $(shell go list -m -f '{{.GoVersion}}')
FRONTEND_DEPS_STAMP := node_modules/.btg-lms-frontend-deps

$(FRONTEND_DEPS_STAMP): package.json pnpm-lock.yaml
	pnpm install --frozen-lockfile
	touch $@

install: $(FRONTEND_DEPS_STAMP)

## Produces the real assets required by internal/web's mandatory go:embed.
## It intentionally skips generated API types and type checking so Go-only
## targets can prepare the embed tree without modifying tracked generated code.
frontend-embed: $(FRONTEND_DEPS_STAMP)
	pnpm exec vite build

frontend-build: $(FRONTEND_DEPS_STAMP)
	pnpm build

generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
	pnpm api:types

build: frontend-build
	go build -ldflags "-X main.version=$(shell git describe --tags --always --dirty)" -o btg-lms ./cmd/btg-lms

test: frontend-build
	go test ./...
	pnpm test

test-integration:
	go test -tags=integration ./internal/platform

test-e2e: frontend-build
	pnpm test:e2e

lint: frontend-embed lint-go

lint-go:
	GOTOOLCHAIN=go$(GO_TOOLCHAIN_VERSION) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

check: frontend-build lint-go
	go run ./tools/archcheck
	go vet ./...
	go test ./...
	pnpm test

clean:
	rm -rf internal/web/dist

dev:
	pnpm dev

db-up:
	docker compose up -d db

migrate:
	go run ./cmd/btg-lms migrate

doctor:
	go run ./cmd/btg-lms doctor
