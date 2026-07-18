# audit-service

Audit logs & compliance events (EP-23, L2).

## Structure

Hexagonal layout generated from `tools/new-service`:

- `internal/domain` — `AuditLog` aggregate, validation, and builder.
- `internal/ports` — `Repository` and `Subscriber` ports.
- `internal/application` — `Sink` use case (CloudEvent → immutable audit log) and
  `Query` use case (tenant-scoped reads; platform super admins may read across tenants).
- `internal/adapters/postgres` — Postgres `Repository` with RLS + cursor pagination.
- `internal/adapters/events` — NATS JetStream subscriber (`AURA.>`, durable `audit-sink`).
- `internal/adapters/http` — liveness/readiness probes + the read-only audit query API.
- `cmd/server` — HTTP server: `/health`, `/ready`, and `GET /api/v1/audit-logs`
  (also mounted at the gateway-prefixed alias `GET /api/v1/audit/logs`).
- `cmd/worker` — event sink worker.
- `migrations/0001_init.sql` — `audit_logs` table + RLS.
- `migrations/0002_platform_admin_rls.sql` — platform super-admin RLS bypass for
  cross-tenant reads.

## Query API

`GET /api/v1/audit-logs?limit=25&cursor=<opaque>` implements
`contracts/openapi/audit.v1.yaml` (`listAuditLogs`). Responses use the
`{data, next_cursor}` envelope. `audit.read` permission is required; tenant
actors are scoped to their own tenant, while platform super admins without a
tenant context read across tenants (RLS bypass via `app.is_platform_admin`).
Writes remain event-driven only — audit logs are immutable.

## Run

```bash
cd apps/audit-service
DATABASE_URL=postgres://... go run ./cmd/server
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/worker
```

## Test

```bash
cd apps/audit-service
go test ./...
```
