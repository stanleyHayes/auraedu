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
