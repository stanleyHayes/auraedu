# website-service

Per-school public site content/pages (EP-19, L2).

Implements the Website Service minimal CRUD for `Page` and `Section` aggregates:
- Domain model with validation and patch `ApplyUpdate`
- Postgres adapter with cursor pagination, tenant-scoped filters, and RLS
- HTTP adapter for pages and sections
- Transactional page/section lifecycle outbox with stable event IDs and retry
- Feature-flag gating (`public_website`) and RBAC (`website.read`, `website.manage`)
- Replay-safe worker that atomically provisions a default `home` page, hero section,
  and lifecycle events on `tenant.created.v1`, while also draining the outbox

## Run

```bash
GOFLAGS=-mod=readonly go run ./cmd/website-service server
GOFLAGS=-mod=readonly go run ./cmd/website-service worker
curl localhost:8080/health
```

## Contract

REST: `contracts/openapi/website.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.

## Selectable persistence (AURA-9.12)

Server, worker and `migrate` default to `DATABASE_DRIVER=postgres`. Select
`mongodb` with `MONGODB_URI` pointing at a replica set (including Atlas).
Startup verifies transaction support before readiness. `MONGODB_DATABASE`
defaults to `auraedu_website`; `MONGODB_MAX_POOL_SIZE` defaults to 2.
Invalid driver/configuration values fail closed. `BIND_HOST` optionally limits
the HTTP listener. PostgreSQL migrations remain available.

Mongo scopes every tenant operation and uses transactions for multi-document
changes, with stable embedded outbox IDs for at-least-once delivery.
Run conformance and adversarial tests with:
`TESTCONTAINERS_RYUK_DISABLED=true GOFLAGS=-mod=readonly go test -p 1 -run TestMongo ./internal/adapters/...`.
