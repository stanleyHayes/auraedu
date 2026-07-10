# fees-service

Fee structures, invoices, balances, and receipts (EP-16, L2).

Hexagonal Go service implementing the `FeeStructure` and `Invoice` aggregates with Postgres persistence,
HTTP CRUD endpoints, and CloudEvents over NATS JetStream.

## Run

```bash
cd apps/fees-service
DATABASE_URL=postgres://... go run ./cmd/server
```

## Contract

REST:
- `GET/POST /api/v1/fee-structures`
- `GET/PATCH/DELETE /api/v1/fee-structures/{fee_structure_id}`
- `GET/POST /api/v1/invoices`
- `GET/PATCH/DELETE /api/v1/invoices/{invoice_id}`

Events:
- `fee.assigned.v1`
- `invoice.created.v1`
- `invoice.updated.v1`
- `invoice.deleted.v1`
- `invoice.paid.v1`

Every action enforces: authenticated → tenant → RBAC (`fees.read` / `fees.manage`) → feature-flag (`fees`) → ownership.
