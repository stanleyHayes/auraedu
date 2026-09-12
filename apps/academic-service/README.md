# academic-service

Hexagonal Go service (agent_plan §5). Scaffolded by `make new-service NAME=academic`.

**Status:** academic years, terms, classes, and subjects implemented (AURA-12.2/12.3/12.4):
domain+migrations, repositories, CRUD+HTTP, events, feature-flag gating, tenant-isolation
tests. Academic years, terms, classes, subjects, timetables, and tenant-owned grading scales are implemented.
Promised year, term, class, and subject lifecycle events commit atomically through a
FORCE-RLS outbox; the worker publishes stable event IDs with bounded retries.

## Run
```bash
GOFLAGS=-mod=readonly go run ./cmd/academic-service server
GOFLAGS=-mod=readonly go run ./cmd/academic-service worker
curl localhost:8080/health
```

## Contract
REST: `contracts/openapi/academic.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.

## Selectable persistence (AURA-9.12)

Server, worker and `migrate` select `DATABASE_DRIVER=postgres` by default or
`DATABASE_DRIVER=mongodb`. Mongo requires `MONGODB_URI` pointing at a replica
set (Atlas is supported); startup verifies transaction support before readiness.
`MONGODB_DATABASE` defaults to `auraedu_academic` and
`MONGODB_MAX_POOL_SIZE` defaults to 2. Invalid driver values and missing Mongo
configuration fail closed. `BIND_HOST` optionally restricts the HTTP listener.

Mongo mutations use tenant-bound scopes and replica-set transactions wherever
several documents must commit together. Stable outbox IDs support at-least-once
delivery. PostgreSQL migrations and the existing `DATABASE_URL` path remain.
Mongo conformance and fault/concurrency regressions use real replica-set fixtures:
`TESTCONTAINERS_RYUK_DISABLED=true GOFLAGS=-mod=readonly go test -p 1 -run TestMongo ./internal/adapters/...`.
