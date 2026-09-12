# assessment-service

Assignments, tests, exams, scores (EP-14, L2).

Implemented: CRUD for `Assessment` and `Score` aggregates with Postgres
persistence, cursor pagination, tenant-scoped RLS, RBAC and feature-flag
gating, and transactional lifecycle event delivery over NATS. Assessment and score
mutations commit their contracted events to a FORCE-RLS outbox; the worker publishes
stable IDs with bounded retry. Assignment create/update/delete remain explicit
non-event boundaries; assignment publication is durable.

- `/api/v1/assessments` + `/api/v1/assessments/{id}/scores` — gated on the
  `assessments` feature flag.
- `/api/v1/assignments` (list/create/get/update/delete + `POST .../publish`) —
  assignments are assessments with `type='assignment'` plus `class_ids` and
  `published_at`; gated on the `assignments` feature flag. Publishing emits
  `assignment.published.v1` (contracts/events/assignment.published.v1.json).
- `/api/v1/gradebook?student_id=|class_id=[&academic_year_id=][&subject_id=]` —
  read-only per-subject and overall averages (simple + max-score-weighted)
  computed from recorded scores; gated on the `assessments` flag.

## Run

```bash
cd apps/assessment-service
DATABASE_URL=postgres://... go run ./cmd/assessment-service server
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/assessment-service worker
```

## Test

```bash
cd apps/assessment-service
go test ./...
```

## Selectable persistence (AURA-9.12)

Server, worker and `migrate` commands use `DATABASE_DRIVER` (`postgres` by
default, or `mongodb`). Mongo mode requires `MONGODB_URI`; `MONGODB_DATABASE`
defaults to `auraedu_assessment`. Mongo indexes are initialized before startup,
and each process uses a two-connection pool.

`BIND_HOST` defaults to an empty host (all interfaces). The combined demo sets
`BIND_HOST=127.0.0.1` for internal services; only the gateway is public.

MongoDB must be a replica set or sharded cluster. Startup refuses a standalone
server because parent validation and related writes use transactions. Durable
events stay embedded in their aggregates, and workers publish with stable IDs.
