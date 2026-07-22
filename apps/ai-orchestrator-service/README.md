# AuraEDU AI Orchestrator Service

This service owns two deliberately separate capabilities:

- The public website admissions assistant retrieves only approved tenant knowledge, returns the approved passage with explicit source citations, records idempotent exchanges under RLS with a 90-day retention deadline, refuses unsupported questions, and emits privacy-safe human-escalation events.
- The controlled-action API accepts only server-allowlisted low-risk tools. Policy `2026-07-19.v1` permits one reversible action: assign one CRM lead to one owner. It requires an independent human with `ai.action.approve` and `crm.lead.assign`, denies AI/service reviewers, hashes the immutable payload, executes idempotently, reduces downstream responses to PII-free identifiers, and records every transition in an append-only tenant-RLS ledger.

The assistant deliberately uses an extractive answer policy. A generative provider can be introduced behind the same grounded-answer boundary only after evaluation thresholds, prompt/version controls and provider credentials are in place. Unknown actions fail closed; bulk communication, public publishing, advertising spend, fees, grades, security roles, audit deletion and admission decisions are not executable tools.

Needs-human exchanges and `assistant.question_unanswered.v1` commit together in
a tenant-isolated PostgreSQL outbox. The separately deployed `worker` publishes
stable CloudEvent/JetStream identities with bounded retry. The HTTP process no
longer treats best-effort broker delivery as success, so an admissions
escalation cannot be silently stranded after the visitor receives its handoff
message. Event data contains only session/message identifiers, locale and time;
the visitor's question and answer remain private.

## Controlled action endpoints

- `POST /api/v1/ai/actions` — propose an allowlisted action with an idempotency key.
- `GET /api/v1/ai/actions` and `GET /api/v1/ai/actions/{id}` — inspect the queue and immutable evidence.
- `POST /api/v1/ai/actions/{id}/approve|reject` — independent human decision; approval executes the exact stored payload.
- `POST /api/v1/ai/actions/{id}/retry` — retry only a previously approved failed payload.

Run `tools/smoke/controlled-actions.sh` from the repository root for the real PostgreSQL + CRM proof.
