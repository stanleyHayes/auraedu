# fees-service

Fee structures, invoices, balances, and receipts (EP-16, L2).

Hexagonal Go service implementing fee structures, invoices, currency-safe student balances, and immutable
payment receipts with Postgres persistence, HTTP endpoints, and CloudEvents over NATS JetStream. Its durable
worker consumes `payment.received.v1`, applies each payment exactly once, records overpayments explicitly, and
publishes invoice lifecycle events from a tenant-isolated transactional outbox. The receipt, balance/invoice
change, and `invoice.updated.v1`/`invoice.paid.v1` records commit together; the worker publishes those records
with stable IDs and retries broker failures without replaying the financial mutation.
Invoice creation, meaningful updates, paid transitions and deletion use that same
outbox: the invoice mutation and its `fee.assigned.v1`/invoice lifecycle events
commit together before the worker publishes them.

## Run

```bash
cd apps/fees-service
DATABASE_URL=postgres://... go run ./cmd/server
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/fees-service worker
```

## Contract

REST:
- `GET/POST /api/v1/fee-structures`
- `GET/PATCH/DELETE /api/v1/fee-structures/{fee_structure_id}`
- `GET/POST /api/v1/invoices`
- `GET/PATCH/DELETE /api/v1/invoices/{invoice_id}`
- `GET /api/v1/balances/{student_id}`
- `GET /api/v1/receipts/{receipt_id}`

Events:
- `fee.assigned.v1`
- `invoice.created.v1`
- `invoice.updated.v1`
- `invoice.deleted.v1`
- `invoice.paid.v1`

Consumed events:
- `payment.received.v1`

Every action enforces: authenticated → tenant → RBAC (`fees.read` / `fees.manage`) → feature-flag (`fees`) → ownership.
