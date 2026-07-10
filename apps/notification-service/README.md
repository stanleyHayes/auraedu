# notification-service

Email, SMS, WhatsApp, in-app notifications (EP-18, L2).

## Run

```bash
cd apps/notification-service
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/server
curl localhost:8080/health
```

## Structure

- `internal/domain` — Message, Template, Subscription aggregates.
- `internal/ports` — repository, event publisher, notifier ports.
- `internal/application` — CRUD + send use cases with tenant scope, RBAC and feature flags.
- `internal/adapters` — Postgres, HTTP, eventbus, mock notifiers.
- `cmd/server` — HTTP entrypoint.
- `cmd/worker` — background event consumer.
- `migrations/0001_init.sql` — Postgres schema + RLS.

## Contract

REST: `contracts/openapi/notification.v1.yaml` (managed separately)  
Events: `contracts/events/notification.sent.v1.json`, `notification.failed.v1.json`.
