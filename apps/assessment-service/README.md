# assessment-service

Assignments, tests, exams, scores (EP-14, L2).

Implemented: minimal CRUD for `Assessment` and `Score` aggregates with Postgres
persistence, cursor pagination, tenant-scoped RLS, RBAC and `assessments` feature
flag gating, and domain event publishing over NATS.

## Run

```bash
cd apps/assessment-service
DATABASE_URL=postgres://... go run ./cmd/server
```

## Test

```bash
cd apps/assessment-service
go test ./...
```
