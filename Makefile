.PHONY: install generate build test test-integration test-e2e lint check dev db-up migrate doctor

GOLANGCI_LINT_VERSION := $(shell cat tools/golangci-lint-version)
GO_TOOLCHAIN_VERSION := $(shell go list -m -f '{{.GoVersion}}')

install:
	pnpm install --frozen-lockfile

generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
	pnpm api:types

build:
	pnpm build
	go build -ldflags "-X main.version=$(shell git describe --tags --always --dirty)" -o btg-lms ./cmd/btg-lms

test:
	go test ./...
	pnpm test

test-integration:
	go test -tags=integration ./internal/platform

test-e2e:
	pnpm test:e2e

lint:
	GOTOOLCHAIN=go$(GO_TOOLCHAIN_VERSION) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

check: lint
	go run ./tools/archcheck
	go vet ./...
	pnpm build
	go test ./...
	pnpm test

dev:
	pnpm dev

db-up:
	docker compose up -d db

migrate:
	go run ./cmd/btg-lms migrate

doctor:
	go run ./cmd/btg-lms doctor
