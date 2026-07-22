# Chapter 24: Event Contracts

## Purpose

Make every inter-service fact in AuraEDU explicit, versioned, tenant-aware and mechanically traceable from producer to schema to consumer.

---

## Scope

JSON Schema contracts under `contracts/events`, generated TypeScript validators, Go/Python runtime event names, producer and consumer parity, compatibility policy, examples and contract review. Delivery mechanics are defined in Chapter 15.

---

## Principles

- A runtime event type is an API and receives the same review discipline as an HTTP operation.
- The filename, schema `$id`, `properties.type.const`, producer type and consumer subject are one identity.
- Published versions are immutable. Breaking meaning requires a new version.
- Tenant identity is part of the envelope, never inferred from payload data or subject naming.
- Examples are executable conformance fixtures, not illustrative prose.

---

## Business Rules

- An event describes a completed domain fact in past tense.
- Producers emit only after the fact and its outbox record commit atomically.
- Consumers may update only their own projections and tolerate additive optional fields.
- Payloads contain only the minimum data needed for the declared outcome; PII is excluded unless an approved contract and lawful purpose require it.
- Event retirement names the final producer, consumers, replay window and operational owner.

---

## Technical Rules

- Contract files are named `<domain>.<fact>.v1.json`; the exact runtime type is `<domain>.<fact>.v1`.
- `$id` ends in `/events/<filename>`, `specversion` is `1.0`, and `type`, `source`, `id`, `time`, `tenant_id` and `data` are required. `time` is an RFC 3339 UTC instant at both schema and shared runtime boundaries.
- Both the CloudEvent envelope and `data` are closed objects with `additionalProperties: false` unless an explicit compatibility decision documents otherwise. The contract validator rejects omission of either boundary.
- Every schema includes at least one example that validates against its declared JSON Schema draft.
- Go producers passing a literal to `tenancy.NewCloudEvent` must use an exact contract type. Python and Go consumer declarations must also resolve to an exact versioned contract.
- The shared Go CloudEvent boundary rejects a type without a `.vN` suffix, an empty source, a missing or non-RFC3339 `time`, missing data, malformed JSON data or data whose top-level value is not an object. Producers cannot publish and consumers cannot dispatch around this invariant.
- The serialized CloudEvent envelope must not exceed 1 MiB. Go and Python publisher boundaries reject it before NATS, and consumers enforce the same ceiling before parsing or dispatch. Contract data is never silently truncated.
- `tools/codegen/src/validate-events.ts` scans production Go/Python source and fails on an unversioned producer, unversioned consumer, runtime-only versioned type, duplicate type or invalid example.
- Producer adapters pass the final versioned type directly to `tenancy.NewCloudEvent`; stripping `.v1` and restoring it later is forbidden because it bypasses construction-time identity.
- When one fact can be published directly in development and through an outbox in production, both paths call one event-data builder. Duplicating dictionary or map construction between those paths is prohibited.
- Durable publication uses the outbox row ID as both the CloudEvent `id` and the JetStream `Nats-Msg-Id` deduplication key; changing only the envelope ID does not provide broker-level replay safety. `Nats-Msg-Id` is transport metadata and must not be serialized as an undeclared `data` or envelope property.
- Raw provider, transport and dependency error strings do not cross the event boundary. Public failure facts carry a bounded stable outcome code; detailed diagnostics remain in the owning service's tenant-scoped record and redacted telemetry.
- Generated artifacts are changed only through `make contracts` or the codegen package; generated files are never hand-edited.
- Contract changes are additive within a version. Removing or renaming a field, changing units, narrowing accepted values or changing semantic meaning requires a new version.

---

## Architecture

```text
contracts/events/<type>.v1.json
        | validate schema + example
        | generate runtime types/validators
        v
producer outbox -- exact <type>.v1 --> AURA.<type>.v1
                                            |
                                            v
                                  exact consumer declaration
                                            |
                                            v
                                 validate CloudEvent envelope
```

The schema repository is authoritative. Producer adapters, transactional outboxes and consumer subscription lists are runtime references to that authority; they do not define alternate names.

---

## Best Practices

- Start with the stakeholder fact and its owning service, then author the schema before implementation.
- Use stable domain identifiers and timestamps; state units in names or descriptions.
- Keep `source` constrained to the owning service.
- Add a producer conformance test and at least one consumer dispatch/idempotency test.
- Load the authoritative schema in producer tests and validate the complete serialized envelope. Checking only required field names is insufficient because it does not prove type/source constants, formats, enums, bounds or nested rules. Transport-capture tests must inspect the serialized envelope rather than only asserting that a publish method was called.
- Every service that constructs production CloudEvents must use the shared recursive Go assertion or Draft 2020-12 Python assertion against a captured transport payload. Consumer-only services do not count as producers merely because their integration tests create fixture events. The contract validator derives both language inventories from production source and fails CI when coverage is missing.
- Include a producer size-boundary regression proving an oversized envelope never reaches its transport.
- Run event validation, code generation, generated-package build and affected service tests together.
- During a version migration, subscribe to both versions only through a time-bounded documented compatibility adapter; do not emit an unversioned alias.

---

## Examples

`contracts/events/lead.created.v1.json` declares `lead.created.v1`. CRM therefore publishes `lead.created.v1`; Analytics and Notification subscribe to `AURA.lead.created.v1`. The validator rejects `lead.created` in either `NewCloudEvent(...)` or a subscription list because it would route to a different JetStream subject.

Creating a student record is not the same fact as enrolling that student. An unassigned create or import emits `student.created.v1`; `student.enrolled.v1` is emitted only when both the class and academic-year identifiers exist.

---

## Anti-patterns

- Publishing `lead.created` while the repository contains only `lead.created.v1.json`.
- Keeping unversioned consumer aliases indefinitely as undocumented compatibility behavior.
- Reusing one type string after changing field meaning or measurement units.
- Putting email addresses, phone numbers, tokens or complete domain objects into events for convenience.
- Treating a schema-valid example as proof that the actual producer emits the same shape.
- Publishing a stronger business fact such as “enrolled” when only a weaker record-creation fact has occurred.
- Building separate direct-publish and outbox payload dictionaries that can drift while retaining the same event type.
- Publishing raw SMTP, payment-gateway, storage or upstream error text that can contain addresses, tokens, URLs or provider internals.
- Overwriting a generated CloudEvent ID for an outbox record without also setting the broker deduplication key.
- Relying only on NATS maximum-payload configuration or truncating contracted data to fit it.
- Hand-editing generated validators or shared event types.

---

## Checklist

- Does filename, `$id` and `type.const` identify the same version?
- Are all required CloudEvent and tenant fields present?
- Do both the envelope and `data` reject undeclared fields?
- Is `time` present and RFC 3339-valid before publication?
- Does every example validate under the declared draft?
- Does the producer emit the exact versioned type transactionally?
- Do direct and outbox publication use one tested event-data builder?
- Does durable publication prove the same stable ID in the envelope and broker deduplication header?
- Are failure outcomes represented by privacy-safe codes instead of raw provider errors?
- Does every consumer subscribe to an exact existing contract?
- Is payload minimization and PII treatment reviewed?
- Does the serialized envelope remain within 1 MiB, with an oversized no-publication regression?
- Are additive compatibility and versioning decisions explicit?
- Do codegen, producer, consumer and idempotency tests pass?

---

## Definition of Done

- The schema and examples pass `validate:events`.
- Generated TypeScript validators/types compile and contain no manual edits.
- Runtime source contains no unversioned producer or consumer alias for a versioned contract.
- Producer tests prove the emitted type, RFC 3339 event time and envelope plus pre-broker size rejection; consumer tests prove dispatch, tenant context, retry and duplicate safety.
- Every production Go CloudEvent constructor is represented by a schema-backed fake-transport test. Every Python service using the production bounded encoder validates its exact direct and outbox envelopes with the shared Draft 2020-12 and format-checking assertion.
- The affected event is documented in Chapter 15 operational flows and has an observable failure path.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Contracts](../../../contracts/README.md)
- [Event contract validator](../../../tools/codegen/src/validate-events.ts)
- [Event-driven architecture](../02-architecture/15-event-driven-architecture.md)
