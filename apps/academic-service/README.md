# academic-service

Hexagonal Go service (agent_plan §5). Scaffolded by `make new-service NAME=academic`.

**Status:** academic years, terms, classes, and subjects implemented (AURA-12.2/12.3/12.4):
domain+migrations, repositories, CRUD+HTTP, events, feature-flag gating, tenant-isolation
tests. Curriculum and grading scales are later stories (AURA-12.9).

## Run
```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/academic-service
curl localhost:8080/health
```

## Contract
REST: `contracts/openapi/academic.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
