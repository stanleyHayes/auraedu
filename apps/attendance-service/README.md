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
