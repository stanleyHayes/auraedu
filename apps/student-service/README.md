# student-service

Hexagonal Go service (agent_plan §5) for tenant-scoped students, guardians,
identity-linked learner scope, class rosters, imports, and durable enrollment history.
Creating a class-assigned student records the initial enrollment atomically; later
`POST /api/v1/students/{student_id}/enrollments` assignments retain academic-year
history while updating the student's current roster projection.

Student, enrollment, guardian, and guardian-link mutations atomically enqueue their
promised integration events in a FORCE-RLS outbox. Run the `worker` command beside
the API to publish stable event IDs with bounded exponential retries.

## Run
```bash
GOFLAGS=-mod=readonly go run ./cmd/student-service server
GOFLAGS=-mod=readonly go run ./cmd/student-service worker
curl localhost:8080/health
```

## Contract
REST: `contracts/openapi/student.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
