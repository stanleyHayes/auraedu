# audit-service

Audit logs & compliance events (EP-23, L2).

## Structure

Hexagonal layout generated from `tools/new-service`:

- `internal/domain` — `AuditLog` aggregate, validation, and builder.
- `internal/ports` — `Repository` and `Subscriber` ports.
- `internal/application` — `Sink` use case: CloudEvent → immutable audit log.
- `internal/adapters/postgres` — Postgres `Repository` with RLS + cursor pagination.
- `internal/adapters/events` — NATS JetStream subscriber (`AURA.>`, durable `audit-sink`).
- `internal/adapters/http` — liveness/readiness probes only (no CRUD in AURA-23.1).
- `cmd/server` — HTTP server with `/health` and `/ready`.
- `cmd/worker` — event sink worker.
- `migrations/0001_init.sql` — `audit_logs` table + RLS.

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
