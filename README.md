# AuraEDU

**Multi-tenant, feature-flag-configurable SaaS school operating system.** One platform, many
schools (tenants) — each with its own branded portal, users, academic structure, and enabled
features. Built as Go microservices + Python AI services + a Next.js web app + an Expo mobile app.

> UPSHS and Aboom AME Zion C Basic School are the **first two tenants**, not separate apps.

## Docs
| Doc | What |
|---|---|
| [`docs/README.md`](docs/README.md) / [`docs/SUMMARY.md`](docs/SUMMARY.md) | **AuraEDU Engineering Handbook** — primary product and engineering source of truth |
| [`agent_plan.md`](agent_plan.md) | Sprints, epics, stories, lanes, Definition of Done — how the work is built in parallel |
| [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md) | Mandatory UI/UX + animation spec (theming, sidebar, tour, mobile) |
| [`docs/deployment/vercel.md`](docs/deployment/vercel.md) | Vercel project boundaries, required environment, and frontend deployment setup |
| [`CLAUDE.md`](CLAUDE.md) / [`AGENTS.md`](AGENTS.md) | Non-negotiable rules + lane ownership |
| [`AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md`](AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md) | Legacy migration source retained for traceability |

## Stack
Go 1.26.5 (hexagonal) · Python 3.14.6/FastAPI · Next.js 16 + React 19 · Expo/React Native · Postgres 18 (DB-per-service + RLS) · NATS JetStream · **Render** (backend) · **Vercel** (frontends) · **Cloudinary** (media) · Render Key Value (Redis).

## Repo layout
```
apps/            web, marketing, mobile, api-gateway, <domain>-service (Go), ai-* (Python)
packages/        ui, ui-native, tokens, shared-types, feature-flags, api-client, config, logger
platform/        shared Go libs (tenancy, auth, flags, httpx, eventbus, db, observ, config, testkit)
contracts/       SOURCE OF TRUTH — openapi/, events/, permissions/, features/
infrastructure/  docker/, render/        deploy/  docker-compose (local)
tools/           codegen, new-service, seed        docs/  architecture, api, onboarding, ai, runbooks
render.yaml      Render Blueprint      eas.json  Expo build/submit
```

## Quick start
```bash
make bootstrap     # install toolchains + workspace deps (pnpm, go work, uv)
make infra-up      # start Postgres/Redis/NATS/OTel via docker compose
make dev           # run the stack
make new-service NAME=student   # scaffold a new hexagonal Go service
```

### Verify the toolchain (Sprint 0)
```bash
GOFLAGS=-mod=readonly go build ./apps/api-gateway/... ./platform/...
GOFLAGS=-mod=readonly go run ./apps/api-gateway/cmd/server &
curl localhost:8080/health   # {"service":"api-gateway","status":"ok",...}
```

> **Go note:** this repo is a **Go workspace** (`go.work`). If you've set a global
> `go env -w GOFLAGS=-mod=mod`, direct `go` commands will error (`-mod may only be set to
> readonly…`). Run with `GOFLAGS=-mod=readonly`; all `make` targets already do.

## Status
The platform is under active multi-sprint development. Use the live task board in
[`agent_plan.md`](agent_plan.md#1a-agent-task-board-live) for current delivery status; do not infer
completion from the legacy specification.
