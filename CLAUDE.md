# CLAUDE.md — AuraEDU operating rules for AI agents & engineers

This file is loaded by AI coding agents. It is the short, enforceable contract.
Full detail lives in [`agent_plan.md`](agent_plan.md) and [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md).

## What AuraEDU is
A **multi-tenant, feature-flag-configurable, microservices SaaS** school operating system.
Schools are **tenants** on one platform — never separate codebases. First tenants: UPSHS
(`upshs`), Aboom AME Zion C Basic (`aboom-ame-zion-c`).

## The 12 non-negotiable rules (agent_plan §2)
1. **One codebase, many tenants.** No school-specific fork/branch/`if tenant == "x"`.
2. **No hardcoded** school names, colours, domains, or logic. Differences = config + feature flags.
3. **Each microservice owns its DB.** No cross-service DB access — REST (sync) or events (async) only.
4. **Every tenant-owned table and event carries `tenant_id`.**
5. **Every protected action checks 4 gates in order:** authenticated → tenant scope → RBAC permission → feature enabled → resource belongs to tenant.
6. **Disabled features blocked in 3 places:** frontend (hidden), API (403 `feature_disabled`), workers (skip).
7. **Hexagonal architecture** inside every service; no business logic in HTTP handlers.
8. **Contracts are law.** Update `contracts/openapi` + `contracts/events` in the *same* PR that changes an interface. Breaking change ⇒ version bump.
9. **Never log PII / sensitive student data.** Structured logs with redaction.
10. **Secure defaults.** Deny by default; secrets only via env / Render env groups.
11. **Tests for tenant isolation and feature flags are mandatory**, not optional.
12. **Traceability:** every branch/commit/PR references its `AURA-x.y` key.

## Stack (locked — do not re-litigate)
- Go 1.25 (hexagonal) domain services · Python 3.13 / FastAPI AI services · Next.js 16 web + marketing · Expo/React Native mobile (teacher/parent/student only).
- Postgres 17 (DB-per-service + RLS) · NATS JetStream (CloudEvents) · **Render** deploy (Blueprints) · **Cloudinary** media · Render Key Value (Redis).
- Latest package versions everywhere; Renovate keeps them current.

## Git conventions
- Branch `feature/AURA-<epic>.<story>-slug` · Commit `AURA-<epic>.<story> <what>` · PR title `AURA-<epic>.<story> <summary>`.

## Definition of Done (agent_plan §10)
Tenant-aware + RLS · correct feature flag (API/worker/UI) · RBAC enforced · resource-ownership check · OpenAPI/events updated · unit+integration+contract tests · **tenant-isolation test** · **feature-flag test** · FE handles on/off · logs+audit (no PII) · health/ready green · CI green · staging smoke.

## Before you code
Read your story in `agent_plan.md`, confirm your **lane + owned directory** (`AGENTS.md` §lanes),
build against **published contracts** (not other services' code/DB), and follow `DESIGN_SYSTEM.md` for any UI.

## Local dev
`make bootstrap` then `make dev`. Go: this repo is a **workspace** — if `go build` errors with
`-mod may only be set to readonly…`, run with `GOFLAGS=-mod=readonly` (a global `go env -w GOFLAGS=-mod=mod` conflicts with workspace mode). `make` targets already set this.
