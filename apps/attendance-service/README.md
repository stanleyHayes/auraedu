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

REST: `GET/POST /api/v1/attendance`, `GET/PATCH/DELETE /api/v1/attendance/{attendance_id}`.
Events: `attendance.marked.v1`, `attendance.updated.v1`, `attendance.deleted.v1`.

Every action enforces: authenticated → tenant → RBAC (`attendance.read` / `attendance.mark`) → feature-flag (`attendance`) → ownership.
