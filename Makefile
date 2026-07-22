# AuraEDU — root task orchestrator across the polyglot monorepo (Go + TS + Python).
# See agent_plan.md §3–§4. Every lane uses these targets; CI calls the same ones.

.DEFAULT_GOAL := help
SHELL := /bin/bash

# Go workspace mode rejects a global `-mod=mod`; force readonly so `make` targets
# work regardless of a developer's global `go env -w GOFLAGS=...` (see README).
export GOFLAGS := -mod=readonly
export GOTOOLCHAIN := auto

# Go modules in the workspace (platform + any service with a go.mod).
GO_MODULES := $(shell find platform apps -name go.mod -exec dirname {} \; | sort)

# Prefer the golangci-lint installed via `go install` in GOPATH/bin; fall back
# to PATH. This avoids stale Homebrew builds shadowing a Go-1.26.5-compiled
# binary when running `make lint-go` locally.
GOLANGCI_LINT := $(shell if command -v go >/dev/null 2>&1; then test -x "$$(go env GOPATH)/bin/golangci-lint" && echo "$$(go env GOPATH)/bin/golangci-lint" || command -v golangci-lint; else command -v golangci-lint; fi)

# ---- Toolchain bootstrap ---------------------------------------------------
.PHONY: bootstrap
bootstrap: ## Install all toolchains + workspace deps (JS, Go, Python)
	@echo "==> pnpm install"; corepack enable >/dev/null 2>&1 || true; pnpm install
	@echo "==> go work sync"; go work sync || true
	@echo "==> uv sync"; uv sync || true
	@echo "Bootstrap complete."

# ---- Local development -----------------------------------------------------
.PHONY: local-config
local-config: ## Generate secure, ignored local secrets and migration URL map
	node tools/dev/generate-local-config.mjs

.PHONY: dev
dev: local-config ## Boot the full local stack (infra + backend services + web + marketing)
	docker compose --env-file .env -f deploy/docker-compose.yml up --build -d
	@echo "==> AuraEDU stack is starting up."
	@echo "    Gateway:   $$(docker compose --env-file .env -f deploy/docker-compose.yml port api-gateway 8080)"
	@echo "    Web:       $$(docker compose --env-file .env -f deploy/docker-compose.yml port web 3000)"
	@echo "    Marketing: $$(docker compose --env-file .env -f deploy/docker-compose.yml port marketing 3001)"

.PHONY: dev-down
dev-down: ## Stop the full local stack
	docker compose --env-file .env -f deploy/docker-compose.yml down

.PHONY: dev-verify
dev-verify: ## Verify the complete local topology, restarts, and readiness endpoints
	./tools/dev/verify-local-runtime.sh

.PHONY: infra-up
infra-up: local-config ## Start local infra (Postgres, Redis, NATS, OTel) via docker compose
	docker compose --env-file .env -f deploy/docker-compose.infra.yml up -d

.PHONY: infra-down
infra-down: ## Stop local infra
	docker compose --env-file .env -f deploy/docker-compose.infra.yml down

# ---- Quality ---------------------------------------------------------------
.PHONY: lint
lint: lint-go lint-python lint-web ## Run all linters (Go + Python + TS/Web)

.PHONY: lint-go
lint-go: ## Lint all Go modules (golangci-lint)
	@test -n "$(GOLANGCI_LINT)" || (echo "golangci-lint not found; install with: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2" && exit 1)
	@for d in $(GO_MODULES); do \
		echo "==> golangci-lint $$d"; \
		(cd "$$d" && GOWORK=off $(GOLANGCI_LINT) run --concurrency 1 ./...) || exit 1; \
	done

.PHONY: lint-python
lint-python: ## Lint Python AI services (ruff + mypy + pyright)
	uv run ruff check .
	uv run ruff format --check .
	uv run mypy .
	uv run pyright

.PHONY: lint-web
lint-web: ## Lint TS/JS workspaces (ESLint + Prettier)
	pnpm lint
	pnpm format:check

.PHONY: test
test: test-go test-python test-web ## Run all tests (Go + Python + TS/Web)

.PHONY: test-go
test-go: ## Run all Go module tests
	@for d in $(GO_MODULES); do \
		echo "==> go test $$d"; \
		(cd "$$d" && GOWORK=off go test ./...) || exit 1; \
	done

.PHONY: test-python
test-python: ## Run Python AI service tests
	uv run pytest

.PHONY: test-web
test-web: ## Run TS/JS workspace tests
	pnpm test

.PHONY: typecheck
typecheck: ## Typecheck TS workspaces
	pnpm typecheck

# ---- Contracts -------------------------------------------------------------
.PHONY: contracts
contracts: ## Regenerate types/stubs from contracts/ (OpenAPI + events)
	@echo "==> validating contracts"; $(MAKE) contracts-lint
	@echo "==> installing workspace dependencies"
	pnpm install
	@echo "==> generating OpenAPI/CloudEvents stubs + validators + types"
	pnpm --filter @auraedu/shared-types run generate
	@echo "==> building generated TypeScript package"
	pnpm --filter @auraedu/shared-types run build
	@echo "==> compiling generated Go stubs"
	cd packages/shared-types/gen/go && gofmt -w . && GOWORK=off go build ./...
	@echo "==> contracts generation complete"

.PHONY: contracts-lint
contracts-lint: ## Lint OpenAPI + validate event producer/consumer and runtime route parity
	node tools/codegen/src/normalize-openapi-metadata.ts
	pnpm exec spectral lint --fail-severity warn 'contracts/openapi/*.yaml'
	node tools/codegen/src/validate-events.ts
	node tools/codegen/src/validate-routes.ts

# ---- Scaffolding -----------------------------------------------------------
.PHONY: new-service
new-service: ## Generate a new hexagonal Go service: make new-service NAME=student
	@test -n "$(NAME)" || (echo "usage: make new-service NAME=<service>"; exit 1)
	bash tools/new-service/generate.sh "$(NAME)"

# ---- Migrations / seed -----------------------------------------------------
.PHONY: migrate
migrate: local-config ## Apply every service migration using a chmod-600 service-to-URL secret file
	AURA_MIGRATION_DATABASE_URLS_FILE="$${AURA_MIGRATION_DATABASE_URLS_FILE:-$(CURDIR)/.auraedu-local/migration-database-urls.json}" node tools/migrations/orchestrate.mjs

.PHONY: migrate-check
migrate-check: ## Validate migration inventory, sequencing, markers, and executable runners
	node tools/migrations/orchestrate.mjs --check

.PHONY: seed
seed: ## Seed the two initial tenants (UPSHS, Aboom)
	go run ./tools/seed

# ---- Infra validation ------------------------------------------------------
.PHONY: compose-validate
compose-validate: ## Validate docker compose files with `docker compose config`
	docker compose -f deploy/docker-compose.infra.yml config >/dev/null
	docker compose -f deploy/docker-compose.yml config >/dev/null
	@echo "==> docker compose files are valid"

.PHONY: smoke-onboarding-activation
smoke-onboarding-activation: ## Prove approval, invite, outage recovery, tenant activation, and admin login
	./tools/smoke/onboarding-activation.sh

# ---- CI convenience --------------------------------------------------------
.PHONY: ci-check
ci-check: lint test contracts compose-validate ## Run the local subset of CI gates

.PHONY: release-evidence-validate
release-evidence-validate: ## Validate release evidence structure, hashes, and ledger parity
	node --test tools/release/verify-readiness.test.mjs
	node tools/release/verify-readiness.mjs

.PHONY: release-readiness
release-readiness: ## Fail unless every production release evidence item is verified
	node tools/release/verify-readiness.mjs --assert-ready

# ---- Help ------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
