# attendance-service

Daily & subject attendance (EP-13, L2).

Hexagonal Go service implementing the `AttendanceRecord` aggregate with Postgres persistence,
HTTP CRUD endpoints, and CloudEvents over NATS JetStream.

## Run

```bash
cd apps/attendance-service
DATABASE_URL=postgres://... go run ./cmd/server
```

## Contract

REST: `GET/POST /api/v1/attendance`, `GET/PATCH/DELETE /api/v1/attendance/{attendance_id}`,
`POST /api/v1/attendance/bulk` (mark a whole class for a date; all-or-nothing validation,
idempotent upsert on `(tenant_id, student_id, academic_year_id, date)`).
Events: `attendance.marked.v1`, `attendance.updated.v1`, `attendance.deleted.v1`.

Attendance writes and their promised integration events commit atomically through a
FORCE-RLS transactional outbox. Run `attendance-service worker` alongside the API to
publish pending events to JetStream with stable event IDs and bounded retries.

Every action enforces: authenticated → tenant → RBAC (`attendance.read` / `attendance.mark`) → feature-flag (`attendance`) → ownership.

## Selectable persistence (AURA-9.12)

Server, worker and `migrate` select `DATABASE_DRIVER=postgres` by default or
`DATABASE_DRIVER=mongodb`. Mongo requires `MONGODB_URI` pointing at a replica
set (Atlas is supported); startup verifies transaction support before readiness.
`MONGODB_DATABASE` defaults to `auraedu_attendance` and
`MONGODB_MAX_POOL_SIZE` defaults to 2. Invalid driver values and missing Mongo
configuration fail closed. `BIND_HOST` optionally restricts the HTTP listener.

Mongo mutations use tenant-bound scopes and replica-set transactions wherever
several documents must commit together. Stable outbox IDs support at-least-once
delivery. PostgreSQL migrations and the existing `DATABASE_URL` path remain.
Mongo conformance and fault/concurrency regressions use real replica-set fixtures:
`TESTCONTAINERS_RYUK_DISABLED=true GOFLAGS=-mod=readonly go test -p 1 -run TestMongo ./internal/adapters/...`.
