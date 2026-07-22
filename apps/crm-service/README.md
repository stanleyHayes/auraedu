# CRM Service

AuraEDU Growth recruitment leads, consent and interaction timelines (`AURA-56`).

## Boundaries

- Owns prospects, lead lifecycle, assignments, consent snapshots, interactions and controlled prospect feedback.
- Does not own applications, enrolled students, messages, campaign spend or analytics projections.
- Every record and query is tenant-scoped; PostgreSQL RLS is mandatory.
- Public capture is idempotent and deduplicates normalized email/phone within one tenant only.
- PII is never emitted in events or logs.
- The notification worker resolves a welcome recipient through the token-protected internal endpoint; it receives an address only when current email consent is true.
- Feedback starts in `pending` review and is never promoted directly into AI prompts or knowledge.

## Run

```bash
DATABASE_URL=postgres://... NATS_URL=nats://... INTERNAL_SERVICE_TOKEN=... go run ./cmd/crm-service server
go run ./cmd/crm-service migrate
go test ./...
```

The gateway must resolve the tenant and rate-limit `POST /api/v1/public/leads` and `POST /api/v1/public/feedback` before production exposure. CRM and the notification worker must share `INTERNAL_SERVICE_TOKEN`.
