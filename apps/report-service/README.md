# report-service

Report cards, transcripts, PDF (EP-15, L2).

Hexagonal Go service (agent_plan §5). Implements CRUD for `ReportTemplate` and
`ReportCard` aggregates, event-driven score/attendance materialization, and
gofpdf report card generation.

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

`POST /api/v1/report-cards/{id}/generate` renders the card synchronously:
tenant header and title/body from the assigned template, student identity,
period, aggregated subject scores table and attendance summary. On success the
card transitions draft → generating → published and emits
`report.published.v1` (payload per `contracts/events/report.published.v1.json`;
`file_url` is the download route). `GET /api/v1/report-cards/{id}/download`
serves the stored file.

## Run

```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/report-service
curl localhost:8080/health
```

## Contract

REST: `contracts/openapi/report.v1.yaml` (TODO) · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
