# staff-service

Hexagonal Go service (agent_plan §5). Scaffolded by `make new-service NAME=staff`.

**Status:** production service spine implemented: tenant-scoped CRUD, identity links,
teacher-to-class/subject assignments, RBAC and feature gates, internal teacher-scope
resolution, RLS, and durable lifecycle events.

Staff create, update, and delete mutations atomically enqueue their matching domain event
in `staff_outbox`. Run the `worker` command alongside the HTTP service to deliver those
events to JetStream with stable event IDs and bounded exponential retry. The worker fails
closed unless both `DATABASE_URL` and `NATS_URL` are configured.

Assignment creation stores the explicit teacher scope and `staff.assigned.v1` in one
transaction. Academic Service combines these class IDs with legacy class-teacher ownership,
so teacher portals and mobile workflows receive one authoritative, fail-closed scope.

## Run
```bash
GOFLAGS=-mod=readonly go run ./cmd/staff-service server
GOFLAGS=-mod=readonly go run ./cmd/staff-service worker
curl localhost:8080/health
```

## Contract
REST: `contracts/openapi/staff.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
