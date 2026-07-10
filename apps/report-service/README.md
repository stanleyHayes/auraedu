# report-service

Report cards, transcripts, PDF (EP-15, L2).

Hexagonal Go service (agent_plan §5). Implements CRUD for `ReportTemplate` and
`ReportCard` aggregates, placeholder PDF generation, and placeholder event
consumption for `assessment.score_recorded.v1` and `attendance.marked.v1`.

## Run

```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/report-service
curl localhost:8080/health
```

## Contract

REST: `contracts/openapi/report.v1.yaml` (TODO) · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
