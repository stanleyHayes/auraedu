# Chapter 15: Event-Driven Architecture

## Purpose

Define how AuraEDU moves durable facts between independently deployed services without losing tenant context, duplicating side effects or allowing one poison event to stall a consumer forever.

---

## Scope

The canonical NATS JetStream topology, CloudEvent envelopes, transactional outboxes, consumer acknowledgement, retries, dead-lettering, idempotency and operational ownership. Request-response APIs and analytics warehouse ingestion are covered only where they meet this boundary.

---

## Principles

- Events are immutable facts, not remote procedure calls.
- A committed business change and its outbox record are one transaction.
- Delivery is at least once; every side effect is idempotent.
- Tenant identity is mandatory and is re-established before repository access.
- Poison input is terminated; temporary dependency failure is retried and then quarantined.

---

## Business Rules

- Publishing success never compensates for an uncommitted domain transaction.
- Consumers may enrich their own projection but must not write another service's database.
- A learner, guardian, teacher or school must not observe duplicate externally visible effects after redelivery.
- Dead-letter entries retain the original event and error context needed for an authorized replay decision.
- Replay is an operator action with an incident or change reference; it is never an automatic infinite loop.

---

## Technical Rules

- Runtime subjects use `AURA.<event-type>` on the canonical `AURA_EVENTS` limits-retention stream. `AURA_DLQ.<event-type>` is isolated on `AURA_DLQ`.
- Envelopes conform to AuraEDU's tenant-aware CloudEvents 1.0 contract and must pass `tenancy.CloudEvent.Validate` before publication or handler execution. Validation requires the `.vN` event identity, source, tenant, ID and JSON-object data.
- Go consumers use `platform/eventbus.Subscribe`; raw production subscriptions are rejected by CI.
- Consumer policy is explicit acknowledgement, a 30-second acknowledgement wait and at most five deliveries. Existing durable consumers are updated in place before binding so deployment does not reset delivery state or fail on configuration drift.
- Python AI consumers set `manual_ack=True`, use explicit acknowledgement, a 30-second wait and five deliveries, and reconcile existing durable configuration before subscribing. Their `AURA_AI` stream is also reconciled to the exact versioned contract subjects, a seven-day maximum age and one-million-message ceiling so an older stream cannot silently omit current producer traffic or grow without bound.
- Producers serialize once and reject an event envelope larger than 1 MiB before calling NATS. `platform/eventbus.Publisher` returns the permanent `ErrEventTooLarge`; each Python AI publisher and outbox uses its service-local shared `encode_event` boundary and raises `EventTooLargeError`. Payloads are never truncated to make them fit.
- Consumers apply the same 1 MiB ceiling. Oversized, malformed or invalid envelopes terminate without reaching domain handlers.
- A successful handler acknowledges. A temporary failure negatively acknowledges with delay. On the fifth failure, the shared Go path publishes to the DLQ and terminates the original only after DLQ publication succeeds.
- Consumers establish tenant context from the validated envelope before data access and retain service-local idempotency claims around side effects.
- Producers use the service-owned transactional outbox. Direct publish after state mutation is prohibited.

---

## Architecture

```text
domain transaction
  |-- business state
  `-- outbox event
          |
          v
    outbox worker --> AURA_EVENTS / AURA.<type>
                           |
                           v
                  durable service consumer
                    | validate envelope + tenant
                    | claim idempotency key
                    | perform local side effect
                    | ack on success
                    ` retry transient failure (max 5)
                           |
                           v
                    AURA_DLQ / operator review
```

The canonical implementation is `platform/eventbus`. Contract schemas live under `contracts/events`; each service owns its outbox table, publisher worker, idempotency state and projection. Recommendation, Prediction and Career Guidance implement the equivalent policy through their `events/subscriber.py` adapters because they use the Python NATS client.

---

## Best Practices

- Use stable event IDs and idempotency keys from the originating transaction.
- Keep handlers small: validate/dispatch in the adapter and execute business work in the application service.
- Log event type and bounded identifiers, never raw payloads or learner data.
- Alert on retries, maximum-delivery advisories, DLQ growth and consumer lag.
- Test new producers with a normal envelope and an oversized envelope that proves the broker receives no call.
- Test new consumers with success, malformed envelope, oversized envelope, transient failure, maximum delivery and replay/idempotency cases.
- Evolve payloads through versioned event types; do not mutate the meaning of a published version.

---

## Examples

```go
subscription, err := eventbus.Subscribe(
    js,
    eventbus.EventStreamName,
    "report-worker-assessment-score-recorded-v1",
    "assessment.score_recorded.v1",
    handleScoreRecorded,
    nil, // canonical JetStream DLQ
)
```

The shared subscriber validates and installs tenant context before calling `handleScoreRecorded`. Returning `nil` acknowledges; returning an error activates bounded retry and DLQ handling.

---

## Anti-patterns

- Calling JetStream `Subscribe` directly in a service and silently inheriting infinite delivery.
- Using automatic acknowledgement while also calling `nak`, which can convert a requested retry into message loss.
- Acknowledging before the local transaction or external side effect is durably complete.
- Retrying malformed JSON, invalid tenant envelopes or missing required fields.
- Deleting and recreating a durable merely to change retry policy; that discards or replays delivery state.
- Publishing a domain event after committing state without an outbox transaction.
- Relying on the broker's configurable maximum payload instead of rejecting an oversized envelope at the application boundary.
- Truncating event data to pass a size limit and thereby changing the contracted fact.
- Treating the DLQ as archival storage and never assigning an operational owner.

---

## Checklist

- Is the event schema versioned, generated and validated?
- Are state and outbox committed atomically?
- Does the consumer use the shared policy or its approved Python equivalent?
- Are tenant context and idempotency established before side effects?
- Does the producer reject an oversized serialized envelope before broker publication?
- Are consumer envelope size, poison input, transient retry and fifth-delivery behavior tested?
- Can existing durable configuration be reconciled without deletion?
- Are retry, lag and DLQ signals observable and owned?
- Is replay documented and authorization-gated?

---

## Definition of Done

- Contract generation and validation pass.
- Producer rollback tests prove state cannot commit without its outbox record.
- Producer tests prove an envelope over 1 MiB returns a permanent size error and never calls NATS.
- Consumer tests prove no handler invocation for malformed or oversized input, acknowledgement only after success and bounded retry with DLQ fallback.
- A broker-backed integration proves the filtered or wildcard durable coexists with other consumers on `AURA_EVENTS`.
- Static security rejects raw Go subscriptions, an unbounded shared Go publisher, incomplete Python AI consumer policy and any AI publisher/outbox that bypasses bounded encoding.
- Operational dashboards and alerts identify lag, retry exhaustion and DLQ growth without exposing payload content.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
- [Eventbus implementation](../../../platform/eventbus/eventbus.go)
- [Alert response runbook](../04-operations/runbooks/alerts.md)
