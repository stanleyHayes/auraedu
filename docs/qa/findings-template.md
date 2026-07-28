# QA findings template

Use one findings document per suspected defect or unexpected behavior. The format forces
evidence before opinion: you may not write a hypothesis until you have walked the code and
cited it, and you may not mark anything RESOLVED without recording the decision so future QA
can see why.

File naming: `docs/qa/findings/YYYY-MM-DD-short-slug.md`. Link findings from the UAT sign-off
block in [uat-checklist.md](uat-checklist.md).

---

## Template (copy everything below this line)

---

# Finding: <one-sentence title>

| Field | Value |
|---|---|
| Finding ID | QA-YYYY-NNN |
| Date raised | |
| Raised by | |
| Environment / commit | |
| Severity | blocker / major / minor / question |
| Status | OPEN / INVESTIGATING / VERDICT-REACHED / RESOLVED |
| Linked UAT section | e.g. uat-checklist.md §9 |

## 1. Observation

What was done, what was expected, what actually happened. Include exact steps, inputs
(tenant, user, IDs), screenshots or response bodies. No interpretation yet — this section is
a replayable recipe.

## 2. Code walk-through

Trace the observed behavior through the code. Cite `path/to/file.go:line` for every claim.
Stop at the point where the observed behavior becomes explainable (or where the trail runs
out — that is itself a fact).

## 3. Hypotheses

Ordered most-likely-first. Add or remove rows as the walk-through dictates.

| # | Hypothesis | How to confirm | Action if true |
|---|---|---|---|
| 1 | | | |
| 2 | | | |
| 3 | | | |

## 4. Verdict

One of:

- **Expected behavior** — the system did what it is designed to do; the checklist item, copy,
  or operator expectation is what needs fixing. Say which.
- **Defect** — the code does not match the intended contract. State the intended contract
  (OpenAPI path, story ID e.g. AURA-x.y, or ledger entry) and the minimal failing case.
- **Needs product decision** — behavior is unambiguous but the intent is not. Name the
  decider.

## 5. Resolution record (keep after RESOLVED)

Never delete this section. A RESOLVED finding is institutional memory: it answers "didn't we
see this before?"

| Field | Value |
|---|---|
| Verdict | |
| Root cause (if defect) | |
| Fix PR / commit | |
| Regression coverage added (test path) | |
| Verified by / date | |
| Follow-up items | |

---

## Filled-in example

# Finding: Replayed Paystack webhook appeared to settle twice in the admin timeline

| Field | Value |
|---|---|
| Finding ID | QA-2026-001 |
| Date raised | 2026-07-28 |
| Raised by | QA lead |
| Environment / commit | staging, candidate `c0ffee1…` |
| Severity | blocker (money rails) |
| Status | RESOLVED |
| Linked UAT section | uat-checklist.md §17 (money rails) |

## 1. Observation

During §17 webhook-idempotency testing on staging (tenant `upshs`, invoice for student
`STU-0042`, GH₵ 850.00), the same Paystack `charge.success` payload with reference
`aura_test_9f31c` was replayed three times via curl against
`POST /api/v1/payments/webhooks/paystack`.

Expected (per checklist): one payment transition, one transaction, one receipt; replays
no-op.

Actual: the admin payment timeline at `/admin/payments` showed the payment row once with
status `succeeded` and **one** receipt — correct — but the webhook audit list
(`/admin/payments` → webhook events) showed the same reference three times with status
`processed`. QA initially read this as a double-settlement defect.

## 2. Code walk-through

- The webhook handler validates the signature and delegates to the processing use case:
  `apps/payment-service/internal/adapters/http/handler.go` (webhook route).
- `Service.ProcessWebhook` records an audit row for every accepted delivery, then checks
  whether the provider reference was already reconciled:
  `apps/payment-service/internal/application/service.go:443` (`ProcessWebhookRequest` /
  processing use case).
- The deduplication read is `WebhookEventRepository.HasProcessedReference`:
  `apps/payment-service/internal/adapters/postgres/repository.go:567`. On `true`, processing
  short-circuits — no payment update, no transaction, no outbox event.
- The single settlement path is `PaymentRepository.CommitReconciliation`
  (`repository.go:223`), which commits payment status + immutable transaction + the
  privacy-safe `payment.received.v1` outbox record in one transaction (AURA-17.11).
- Conclusion of the walk-through: the three audit rows are the intended audit trail of three
  accepted deliveries; the dedup check prevented any ledger effect from deliveries 2 and 3.

## 3. Hypotheses

| # | Hypothesis | How to confirm | Action if true |
|---|---|---|---|
| 1 | `HasProcessedReference` failed to match the replayed reference (e.g. provider/ref column mismatch), so all three deliveries reconciled | Count rows in `payment_transactions` for the payment; check invoice balance credited once; re-run replay against a fresh payment and query the repository directly | Defect in dedup query — fix join/columns, add a replay regression test |
| 2 | Dedup worked, but the webhook audit UI labels every accepted delivery `processed`, indistinguishable from `reconciled` — operator misreads audit trail as settlement trail | Compare `payment_transactions` count (expect 1) with the three audit rows; inspect the status value stored per audit row | UX/copy fix: surface a `duplicate`/`skipped` outcome state; update §17 wording so QA checks receipts/transactions, not audit-row count |
| 3 | Replay protection works but the outbox emitted duplicate `payment.received.v1` events, risking double Fees reconciliation downstream | Inspect the payment outbox table for the reference; check Fees consumer dedup (`SKIP LOCKED` claim + advisory-lock reconcile, AURA-16.11) | Defect in outbox write path — fix atomicity, verify Fees exactly-once |

## 4. Verdict

**Expected behavior, with a UX defect spin-off.** Hypothesis 2 confirmed: exactly one
transaction and one receipt existed (hypotheses 1 and 3 disproven by direct queries). The
webhook audit list displayed all accepted deliveries as `processed` without distinguishing
`reconciled` from `duplicate-skipped`. The money rails behaved per contract; the admin
surface was misleading.

## 5. Resolution record

| Field | Value |
|---|---|
| Verdict | Expected behavior (settlement) + minor UX defect (audit labeling) |
| Root cause (if defect) | Webhook audit list rendered one `processed` state for both reconciled and dedup-skipped deliveries |
| Fix PR / commit | <PR link> — audit list now shows `reconciled` vs `duplicate` |
| Regression coverage added | Payment webhook replay integration test asserting 1 transaction / 1 receipt / N audit rows; web test for the two audit states |
| Verified by / date | QA lead, 2026-07-29, re-run of uat-checklist.md §17 |
| Follow-up items | §17 checklist wording amended to define "no change" as receipts + transactions, not audit-row count |
