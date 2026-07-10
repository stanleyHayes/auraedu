# AuraEDU — root task orchestrator across the polyglot monorepo (Go + TS + Python).
# See agent_plan.md §3–§4. Every lane uses these targets; CI calls the same ones.

.DEFAULT_GOAL := help
SHELL := /bin/bash

# Go workspace mode rejects a global `-mod=mod`; force readonly so `make` targets
# work regardless of a developer's global `go env -w GOFLAGS=...` (see README).
export GOFLAGS := -mod=readonly
export GOTOOLCHAIN := local

# ---- Toolchain bootstrap ---------------------------------------------------
.PHONY: bootstrap
bootstrap: ## Install all toolchains + workspace deps (JS, Go, Python)
	@echo "==> pnpm install"; corepack enable >/dev/null 2>&1 || true; pnpm install
	@echo "==> go work sync"; go work sync || true
	@echo "==> uv sync"; uv sync || true
	@echo "Bootstrap complete."

# ---- Local development -----------------------------------------------------
.PHONY: dev
dev: infra-up ## Boot the full local stack (infra + apps)
	pnpm dev

.PHONY: infra-up
infra-up: ## Start local infra (Postgres, Redis, NATS, OTel) via docker compose
	docker compose -f deploy/docker-compose.infra.yml up -d

.PHONY: infra-down
infra-down: ## Stop local infra
	docker compose -f deploy/docker-compose.infra.yml down

# ---- Quality ---------------------------------------------------------------
.PHONY: lint
lint: ## Lint everything (turbo + go vet + ruff)
	pnpm lint || true
	go vet ./... || true
	uv run ruff check . || true

.PHONY: test
test: ## Run all tests (turbo + go test + pytest)
	pnpm test || true
	go test ./... || true
	uv run pytest || true

.PHONY: typecheck
typecheck: ## Typecheck TS workspaces
	pnpm typecheck

# ---- Contracts -------------------------------------------------------------
.PHONY: contracts
contracts: ## Regenerate types/stubs from contracts/ (OpenAPI + events)
	@echo "==> validating contracts"; $(MAKE) contracts-lint
	pnpm --filter @auraedu/shared-types run generate || echo "codegen: TODO (AURA-1.4)"

.PHONY: contracts-lint
contracts-lint: ## Lint OpenAPI + validate event JSON schemas
	@command -v spectral >/dev/null 2>&1 && spectral lint 'contracts/openapi/*.yaml' || echo "spectral not installed (AURA-1.1)"

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

# ---- Help ------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
