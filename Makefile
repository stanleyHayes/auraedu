# AuraEDU — root task orchestrator across the polyglot monorepo (Go + TS + Python).
# See agent_plan.md §3–§4. Every lane uses these targets; CI calls the same ones.

.DEFAULT_GOAL := help
SHELL := /bin/bash

# Go workspace mode rejects a global `-mod=mod`; force readonly so `make` targets
# work regardless of a developer's global `go env -w GOFLAGS=...` (see README).
export GOFLAGS := -mod=readonly
export GOTOOLCHAIN := local

# Go modules in the workspace (platform + any service with a go.mod).
GO_MODULES := $(shell find platform apps -name go.mod -exec dirname {} \; | sort)

# ---- Toolchain bootstrap ---------------------------------------------------
.PHONY: bootstrap
bootstrap: ## Install all toolchains + workspace deps (JS, Go, Python)
	@echo "==> pnpm install"; corepack enable >/dev/null 2>&1 || true; pnpm install
	@echo "==> go work sync"; go work sync || true
	@echo "==> uv sync"; uv sync || true
	@echo "Bootstrap complete."

# ---- Local development -----------------------------------------------------
.PHONY: dev
dev: ## Boot the full local stack (infra + backend services + web + marketing)
	docker compose -f deploy/docker-compose.yml up --build -d
	@echo "==> AuraEDU stack is starting up."
	@echo "    Gateway:   http://localhost:8080"
	@echo "    Web:       http://localhost:3000"
	@echo "    Marketing: http://localhost:3001"

.PHONY: dev-down
dev-down: ## Stop the full local stack
	docker compose -f deploy/docker-compose.yml down

.PHONY: infra-up
infra-up: ## Start local infra (Postgres, Redis, NATS, OTel) via docker compose
	docker compose -f deploy/docker-compose.infra.yml up -d

.PHONY: infra-down
infra-down: ## Stop local infra
	docker compose -f deploy/docker-compose.infra.yml down

# ---- Quality ---------------------------------------------------------------
.PHONY: lint
lint: ## Lint everything (turbo + go vet + ruff)
	pnpm lint
	@for m in $(GO_MODULES); do echo "==> go vet $$m"; (cd $$m && go vet ./...) || exit 1; done
	@if [ -f uv.lock ]; then uv run ruff check .; else echo "==> no uv.lock; skipping ruff"; fi

.PHONY: test
test: ## Run all tests (turbo + go test + pytest)
	pnpm test
	@for m in $(GO_MODULES); do echo "==> go test $$m"; (cd $$m && go test ./...) || exit 1; done
	@if [ -f uv.lock ]; then uv run pytest; else echo "==> no uv.lock; skipping pytest"; fi

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
contracts-lint: ## Lint OpenAPI + validate event JSON schemas
	@command -v spectral >/dev/null 2>&1 || \
		(echo "spectral not installed; run: npm install -g @stoplight/spectral-cli" && exit 1)
	spectral lint 'contracts/openapi/*.yaml'
	@echo "==> event JSON schema validation: TODO (AURA-1.2)"

# ---- Scaffolding -----------------------------------------------------------
.PHONY: new-service
new-service: ## Generate a new hexagonal Go service: make new-service NAME=student
	@test -n "$(NAME)" || (echo "usage: make new-service NAME=<service>"; exit 1)
	bash tools/new-service/generate.sh "$(NAME)"

# ---- Migrations / seed -----------------------------------------------------
.PHONY: migrate
migrate: ## Run all service DB migrations
	@echo "migrate: TODO (per-service, AURA-2.5)"

.PHONY: seed
seed: ## Seed the two initial tenants (UPSHS, Aboom)
	go run ./tools/seed || echo "seed: TODO (AURA-52.x)"

# ---- Infra validation ------------------------------------------------------
.PHONY: compose-validate
compose-validate: ## Validate docker compose files with `docker compose config`
	docker compose -f deploy/docker-compose.infra.yml config >/dev/null
	docker compose -f deploy/docker-compose.yml config >/dev/null
	@echo "==> docker compose files are valid"

# ---- CI convenience --------------------------------------------------------
.PHONY: ci-check
ci-check: lint test contracts compose-validate ## Run the local subset of CI gates

# ---- Help ------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
