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
