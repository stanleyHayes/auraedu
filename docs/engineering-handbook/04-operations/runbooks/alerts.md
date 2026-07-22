# AuraEDU Alert Runbooks

## First-response rules

1. Acknowledge the alert and identify the current incident commander.
2. Confirm impact from the user-facing path; do not rely only on a single metric.
3. Preserve tenant isolation. Never paste payloads, tokens, email addresses, phone numbers or student identifiers into incident channels.
4. Prefer rollback or traffic isolation over live data repair. Any data repair requires an audited, reviewed command.
5. Record start time, affected services, affected regions or tenants, mitigation, recovery time and follow-up owner.

## AuraEDUServiceDown

Owner: Platform. Severity: critical.

- Check the target's `/ready` result and deployment status. A live `/health` endpoint does not prove its database, NATS or Redis dependency is ready.
- Compare the deployment start time with the first failed scrape. Inspect redacted service logs and the most recent rollout.
- If a new rollout caused the outage, halt promotion and roll back to the last verified image.
- If a dependency is unavailable, follow its recovery procedure and keep writes stopped where partial processing could create inconsistent state.
- Resolve only after five consecutive successful scrapes and a synthetic user-path check.

## AuraEDUHighHTTPErrorRate

Owner: Platform plus the owning service team. Severity: critical.

- Break down errors by `service`, canonical `route` and `status`; never add raw path labels.
- Correlate the first spike with traces and deployment markers, then separate dependency timeouts from application failures.
- Roll back a regression. For dependency saturation, shed optional work and preserve admissions, identity, payments and learning flows.
- Resolve after the five-minute error ratio remains below 1% for ten minutes and the synthetic path succeeds.

## AuraEDUHighP95Latency

Owner: Platform plus the owning service team. Severity: warning.

- Confirm p95 by service and route, then inspect in-flight requests, database pool pressure, slow queries and downstream spans.
- Check whether a background job or tenant-wide export is competing with interactive traffic.
- Apply bounded concurrency or pause non-critical work before scaling. Do not disable timeouts.
- Resolve after p95 remains below 750ms for ten minutes.

## AuraEDUSustainedFailedLogins

Owner: Security. Severity: warning.

- Confirm the rate and source distribution from gateway or edge telemetry without collecting submitted credentials.
- Check rate-limit enforcement, credential-stuffing indicators and whether a single school is targeted.
- Tighten edge controls or block abusive sources; do not weaken MFA or account-lock protections.
- Escalate to the incident-response lead when legitimate accounts show compromise indicators.

## AuraEDUPaymentWebhookFailures

Owner: Payments. Severity: critical.

- Verify provider status, signature validation failures and the idempotency ledger.
- Preserve every signed webhook for supported replay through the audited provider flow; never mutate balances manually from an alert.
- Restore processing, replay only unprocessed events, and reconcile provider transactions against AuraEDU receipts.
- Resolve after the webhook error rate is zero for five minutes and a synthetic or provider-approved test webhook succeeds.

## AuraEDUNotificationAPIFailures

Owner: Communications. Severity: warning.

- Separate API persistence failures from provider delivery failures and inspect queue age.
- Confirm consent and channel configuration before retrying. Never switch a recipient to another external channel without lawful consent.
- Pause bulk campaigns when transactional delivery is degraded.
- Resolve after API errors recover and queued transactional messages drain within the delivery objective.

## AuraEDUAIServiceLatency

Owner: AI Platform. Severity: warning.

- Compare model/provider latency with database and feature-store spans.
- Preserve deterministic product paths; degrade optional AI assistance behind its feature flag when the provider is unhealthy.
- Never bypass human review or substitute an unapproved model to clear the alert.
- Resolve after p95 remains below 2.5 seconds for ten minutes and output-safety checks pass.

## AuraEDUNotificationDeliveryFailures

Owner: Communications. Severity: critical.

- Break down the bounded metric by `channel` and `outcome`; recipient and tenant identifiers must never be added as labels.
- Verify provider status, credentials, quota, consent configuration and queue age. Use provider request IDs from redacted structured logs where available.
- Pause bulk campaigns first. Keep in-app delivery available while an external channel is degraded, but do not silently reroute recipients to a channel without consent.
- Retry only durable failed or pending records through the supported worker path; never mark a message sent without provider evidence.
- Resolve after provider failures remain at zero for five minutes and an approved synthetic delivery succeeds.

## AuraEDUWorkerJobFailures

Owner: Platform plus the owning domain team. Severity: warning.

- Break down failures by the declared `service` and `job` labels, then inspect queue lag and redelivery count.
- Determine whether the failure is transient, poison input or a deployment regression. Runtime event values are intentionally collapsed to `unknown` and must not be promoted to labels.
- Keep at-least-once semantics: acknowledge only completed work, quarantine poison messages through the supported dead-letter path, and preserve idempotency records.
- Roll back regressions or restore the failed dependency. Never skip a failed job merely to clear the alert.
- Resolve after the job failure rate is zero for five minutes and queue lag returns to its objective.
