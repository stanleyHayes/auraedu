# cbt-service

Computer-based / online exams (EP-24, L2).

Implements minimal CRUD for `QuestionBank`, `ExamSession` and `Submission`
aggregates with tenant-scoped Postgres persistence, RBAC, feature-flag gating
and domain-event publishing.

Question and exam create/update/delete plus submission/grading events commit atomically
through a FORCE-RLS transactional outbox. Run `cbt-service worker` beside the API to
publish pending events with stable IDs and bounded retries. Starting a submission is an
explicit non-event boundary because no versioned event contract exists for that action.

## Run

```bash
cd apps/cbt-service
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/server
curl localhost:8080/health
curl localhost:8080/ready
```

## Tests

```bash
cd apps/cbt-service
go test ./...
```

Integration tests spin up a Postgres container via `platform/testkit`.

## Contract

REST: `contracts/openapi/cbt.v1.yaml` · Events: `contracts/events/cbt.*.v1.json`.
