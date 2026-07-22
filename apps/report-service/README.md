# report-service

Report cards, transcripts, PDF (EP-15, L2).

Hexagonal Go service (agent_plan §5). Implements CRUD for `ReportTemplate` and
`ReportCard` aggregates, event-driven score/attendance materialization, and
gofpdf report card generation. `GET /api/v1/transcripts/{student_id}` derives a
current transcript from published/archived cards and their score/attendance evidence;
parent and student callers remain constrained to their resolved learner scope.

## Event materialization (AURA-15.9)

The worker (`cmd/worker`) consumes `assessment.score_recorded.v1` and
`attendance.marked.v1` and upserts materialized entries onto the student's
DRAFT report card (auto-created when none exists):

- `report_card_score_entries` — one row per `(report_card_id, source_key)`;
  `source_key` is `assessment_id`, falling back to `score_id`, then the event
  id. Replays and corrections converge on the same row (idempotent).
- `report_card_attendance_entries` — one row per `(report_card_id, entry_date)`;
  re-marks update the day in place.

Draft routing (`FindDraftReportCard`): an event with a `term_id` prefers an
exact-term draft, falling back to a NULL-term draft (period not yet assigned);
an event without a term attaches to the student's most recent draft. Tenants
with the `report_cards` feature disabled are skipped. Malformed events are
acked (dropped); transient failures are nacked for redelivery.

## PDF generation

`POST /api/v1/report-cards/{id}/generate` atomically moves the card to
`generating`, writes a replay-safe PostgreSQL job and returns `202`. The worker
leases ready jobs with `FOR UPDATE SKIP LOCKED`, renders tenant header,
template, student/period, aggregated scores and attendance, then stores the PDF
in tenant-scoped object storage. Render production fails closed unless
`REPORT_STORAGE_BACKEND=cloudinary` and `CLOUDINARY_URL` is configured; local
Compose uses a shared named volume. Failed jobs use bounded exponential retry,
expired leases are reclaimed after worker crashes, and terminal failure returns
the card to `draft` for an explicit human retry.

Every template and report-card create, update and delete also commits its
contracted lifecycle event to the tenant-isolated transactional outbox in the
same database transaction. Publication likewise changes the card and job and
writes `report.published.v1` in one commit. The worker dispatches every outbox
record with a stable event ID and bounded exponential retry, so a broker outage
cannot lose a committed lifecycle transition (publication payload per
`contracts/events/report.published.v1.json`; `file_url` is the authorized
download route). `GET
/api/v1/report-cards/{id}/download` streams the stored file through report RBAC
and learner ownership checks. Cloudinary assets use authenticated delivery with
a short-lived server-generated signature; storage keys are excluded from REST
DTOs and events.

## Run

```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/report-service
curl localhost:8080/health
```

## Contract

REST: `contracts/openapi/report.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
