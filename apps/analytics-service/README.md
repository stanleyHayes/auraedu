# Analytics service

The tenant-isolated read model for operational KPIs, assessment projections and Growth executive analytics (EP-21). Event workers project contract events into idempotent facts; the HTTP process serves permission-scoped views without reading another service's database.

## HTTP API

- `GET /api/v1/analytics/metrics` — cursor-paginated metrics with name, date and dimension filters.
- `GET /api/v1/analytics/executive/growth` — funnel, campaign and conversion summary for a date window.
- `POST /api/v1/analytics/executive/query` — bounded executive question over the tenant's projected data.
- `/healthz` and `/readyz` — liveness and PostgreSQL readiness.

Requests arrive through the API Gateway with authenticated actor and tenant headers. Teacher results are narrowed through Student Service's internal learner-scope endpoint. Feature gates and permissions are enforced in the application layer; PostgreSQL row-level security remains the final tenant boundary.

## Runtime

Required configuration is `DATABASE_URL`. Production wiring also provides `NATS_URL`, `SERVICE_TENANT_URL`, `SERVICE_STUDENT_URL`, `INTERNAL_SERVICE_TOKEN`, and the shared feature registry. Run the server or worker through the service CLI under `cmd/analytics-service`.

## Verification

```sh
go test ./...
go vet ./...
```

Integration tests use disposable PostgreSQL and NATS containers and therefore require Docker.

## Selectable persistence (AURA-9.12)

Server, worker and `migrate` select `DATABASE_DRIVER=postgres` by default or
`DATABASE_DRIVER=mongodb`. Mongo requires `MONGODB_URI` pointing at a replica
set (Atlas is supported); startup verifies transaction support before readiness.
`MONGODB_DATABASE` defaults to `auraedu_analytics` and
`MONGODB_MAX_POOL_SIZE` defaults to 2. Invalid driver values and missing Mongo
configuration fail closed. `BIND_HOST` optionally restricts the HTTP listener.

Mongo mutations use tenant-bound scopes and replica-set transactions wherever
several documents must commit together. Stable outbox IDs support at-least-once
delivery. PostgreSQL migrations and the existing `DATABASE_URL` path remain.
Mongo conformance and fault/concurrency regressions use real replica-set fixtures:
`TESTCONTAINERS_RYUK_DISABLED=true GOFLAGS=-mod=readonly go test -p 1 -run TestMongo ./internal/adapters/...`.
