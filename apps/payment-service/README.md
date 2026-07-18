# payment-service

Payment gateway integrations & webhooks (EP-17, L2).

Hexagonal Go service: `internal/domain` (aggregates), `internal/ports` (boundaries),
`internal/application` (use cases: tenant scope + RBAC + feature flags),
`internal/adapters` (HTTP, Postgres, provider gateways, events).

## Provider configuration

The gateway adapter is selected at boot via env:

| Env | Default | Notes |
|---|---|---|
| `PAYMENTS_PROVIDER` | `mock` | `mock` (deterministic, local/dev) or `paystack` |
| `PAYSTACK_SECRET_KEY` | — | Required when `PAYMENTS_PROVIDER=paystack`. Sent only as `Authorization: Bearer …`; never logged |
| `PAYSTACK_BASE_URL` | `https://api.paystack.co` | Override for tests/staging |

The Paystack adapter (`internal/adapters/provider/paystack.go`) implements the same
`ports.PaymentProvider` interface as the mock:

- **Initiate** → `POST /transaction/initialize`. The payment ID is sent as the Paystack
  reference (deterministic reconciliation), `amount_cents` maps to Paystack subunits,
  and `{payment_id, tenant_id, invoice_id}` ride in `metadata`. A payer email is picked
  up from payment metadata key `email` when present (Paystack expects one).
- **Verify** → `GET /transaction/verify/:reference`. Paystack `success` maps to domain
  `success`; every other state maps to `failed` (the port's binary semantics).

Each call has a 10 s timeout, responses are size-capped, and failures surface as
structured `*provider.Error{Op, StatusCode, Message}` — the secret never appears in errors.

## Webhook setup

`POST /api/v1/webhooks/{provider}` (`paystack` | `flutterwave` | `mock`) receives
provider callbacks. The raw body is verified **before any processing**:

- **Paystack** — hex `HMAC-SHA512(rawBody, secret)` compared (constant-time) to the
  `X-Paystack-Signature` header. Secret from `PAYSTACK_WEBHOOK_SECRET`, falling back to
  `PAYSTACK_SECRET_KEY` (Paystack signs with the API secret key).
- **Flutterwave** — the `verif-hash` header must equal `FLUTTERWAVE_WEBHOOK_SECRET`
  (constant-time compare).

Invalid signatures are rejected with **401**. When no secret is configured for the
provider, the webhook is accepted (dev mode) with a warning log — set the secrets in
any real deployment. Configure the provider dashboard to post to
`https://<host>/api/v1/webhooks/paystack` (or `/flutterwave`).

Payloads are understood in the generic shape (`{reference, tenant_id}`), the Paystack
shape (`{event, data: {reference, metadata: {tenant_id}}}`) and the Flutterwave charge
shape (`{event, data: {tx_ref}}`). The tenant is taken from the verified payload and
scopes all DB access (RLS).

## Reconciliation semantics

- **Idempotent processing** — a processed-events guard keyed on
  `(tenant, provider, reference)` (`webhook_events` table) short-circuits redelivered
  webhooks: the duplicate is recorded for audit but state is never re-applied (no double
  transactions, no duplicate events).
- **Transitions only** — `pending/processing → success` sets `completed_at`, records a
  `credit/success` transaction and emits `payment.received.v1`; `→ failed` records a
  `debit/failed` transaction and emits `payment.failed.v1`. Already-applied outcomes are
  no-ops; `success` is absorbing (a late failure never regresses a received payment); a
  `failed` payment may be corrected to `success` (reconciliation fix-up).
- **Manual verify** — `GET /api/v1/payments/{id}/verify` (perm `payments.initiate`)
  calls the provider's verify endpoint and reconciles through the same path. Use it to
  heal payments stuck `processing` after a lost webhook.

Event payloads conform to `contracts/events/payment.received.v1.json` /
`payment.failed.v1.json` (`payment_id`, `invoice_id`, `amount`, `gateway`, …).

## Development

```sh
cd apps/payment-service
GOWORK=off GOFLAGS=-mod=readonly go build ./...
go vet ./...
go test ./...          # unit + testcontainers integration (needs docker)
```

Run locally: `DATABASE_URL=… PAYMENTS_PROVIDER=mock go run ./cmd/payment-service server`
(migrations auto-apply from `migrations/`). See `deploy/` for the full infra stack.
