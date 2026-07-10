# student-service

Hexagonal Go service (agent_plan §5). Scaffolded by `make new-service NAME=student`.

**Status:** skeleton — health + wiring compile. Implement the 8-story spine (agent_plan §16):
domain+migrations, repository, CRUD+HTTP, events published/consumed, feature-flag gating,
tenant-isolation tests, observability+audit.

## Run
```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/student-service
curl localhost:8080/health
```

## Contract
REST: `contracts/openapi/student.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
