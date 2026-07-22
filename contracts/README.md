# contracts/ — the source of truth for every interface

**Nothing is built against another service's code or database — only against these contracts**
(agent_plan §6). This is what lets 15+ agents work in parallel.

## Layout
- `openapi/<service>.v1.yaml` — REST APIs (OpenAPI 3.1), one per service.
- `events/<event>.v1.json` — domain events (JSON Schema + CloudEvents 1.0 envelope). Every event **must** carry `tenant_id`.
- `permissions/permissions.yaml` — RBAC permission keys + role scopes (spec §8).
- `features/features.yaml` — feature-flag keys + plan mapping + seed defaults (spec §3, §9).

## Change process (mandatory order)
1. **Contract PR first.** Add/modify the `openapi`/`events` file. Merge it *before* implementation.
2. **Generate.** `make contracts` runs codegen → Go server/client stubs, TS types (`packages/shared-types`), JSON-schema validators, the Go authorization registry, and typed Go/TS feature registries. Generated output is committed.
3. **Implement producer + consumers in parallel**, each against the generated stub/types + a local mock.
4. **Contract tests** run on both sides in CI and fail the build on drift.

## Versioning
- Version in the path/topic: `/api/v1/...`, event `assessment.score_recorded.v1`.
- **Additive** (new optional field) → no bump.
- **Breaking** → new `v2` file; keep the old until all consumers migrate.

## Ownership
`contracts/` is owned by **lane L0** (CODEOWNERS). All contract PRs require L0 review so two
lanes never silently diverge on a shared shape.

## Standard error envelope (all services)
`{ "code": "...", "message": "...", "request_id": "..." }` with canonical codes:
`forbidden`, `feature_disabled`, `tenant_mismatch`, `validation_error`, `not_found`, `unauthorized`.

## Seeded so far (Sprint 0)
- `openapi/tenant.v1.yaml` (incl. the pivotal `GET /features` snapshot endpoint)
- `events/tenant.created.v1.json`, `events/assessment.score_recorded.v1.json`
- `permissions/permissions.yaml`, `features/features.yaml`
The rest of the OpenAPI + event skeletons land in EP-01 (AURA-1.1, AURA-1.2).
