# AGENTS.md — lanes, ownership & conventions

Companion to [`CLAUDE.md`](CLAUDE.md). This file defines **who owns what** so many agents
work in parallel without collisions (agent_plan §8).

## Lanes (own these directories)

| Lane | Owns | Skills |
|---|---|---|
| **L0 Platform** | `platform/*`, `contracts/*`, `tools/*`, `packages/{shared-types,feature-flags,api-client,config,logger}` | Go, contracts, codegen |
| **L1 Gateway/Identity/Tenant** | `apps/{api-gateway,identity-service,tenant-service}` | Go, auth, security |
| **L2 Domain-Go** | one agent per `apps/<x>-service` | Go hexagonal |
| **L3 AI-Python** | `apps/ai-*`, `apps/career-guidance-service`, `docs/ai` | Python, ML, FastAPI |
| **L4 Frontend** | `apps/web`, `apps/marketing`, `packages/{ui,tokens}` | Next.js, React, TS |
| **L5 Infra/DevX** | `infrastructure/*`, `deploy/*`, `render.yaml`, `.github/workflows`, `Makefile` | Docker, Render, CI/CD |
| **L6 Quality** | cross-service `tests/`, `platform/testkit`, security/perf | QA, security |
| **L7 Mobile** | `apps/mobile`, `packages/{ui-native,tokens}`, `eas.json` | Expo, React Native, NativeWind |

## Collision-avoidance rules
1. **One owner per service directory per sprint** (see agent_plan Appendix D).
2. **Shared code changes are their own PRs** — touching `platform/*`, `packages/*`, `contracts/*` is a separate ticket reviewed by the owning lane, never bundled into a feature story.
3. **`CODEOWNERS` enforces it** — GitHub blocks merge without the owning lane's review.
4. **Generated files are committed, never hand-edited** (`*.gen.go`, `packages/shared-types/*`) — change only via `make contracts`.
5. **Migrations are additive & service-local** (`apps/<x>-service/migrations/`).

## Contracts-first (agent_plan §6)
Never build against another service's code or DB. Author/modify the contract
(`contracts/openapi/*.yaml`, `contracts/events/*.json`) in a **contract PR first**, run
`make contracts` to regenerate stubs/types, then implement producer + consumers in parallel.

## Conventions
- **Go services:** hexagonal layout (agent_plan §5). Generate with `make new-service NAME=<x>`.
- **Frontend:** build to `DESIGN_SYSTEM.md`; consume `packages/ui`/`ui-native`; never fork shared components.
- **Every story:** references its `AURA-x.y` key in branch/commit/PR and meets the Definition of Done.

## Where things are
- Engineering handbook: `docs/README.md` + `docs/SUMMARY.md` · Plan & backlog: `agent_plan.md` · Design: `DESIGN_SYSTEM.md` · Legacy source: `AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md`.
- Feature flag keys: `contracts/features/features.yaml` · Permissions: `contracts/permissions/permissions.yaml` · Events: `contracts/events/`.
