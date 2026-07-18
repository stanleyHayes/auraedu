# assessment-service

Assignments, tests, exams, scores (EP-14, L2).

Implemented: CRUD for `Assessment` and `Score` aggregates with Postgres
persistence, cursor pagination, tenant-scoped RLS, RBAC and feature-flag
gating, and domain event publishing over NATS.

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
DATABASE_URL=postgres://... go run ./cmd/server
```

## Test

```bash
cd apps/assessment-service
go test ./...
```
