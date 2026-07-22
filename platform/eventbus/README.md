# `platform/eventbus`

NATS JetStream publishing and consumption for AuraEDU CloudEvents.

`Publisher.Publish` validates the tenant-aware envelope before publishing it to a canonical subject. `EnsureStream` reconciles the durable `AURA_EVENTS` work-queue stream. `Subscribe` configures durable consumers with explicit acknowledgement; poison messages are sent to the bounded dead-letter stream through `JetStreamDLQ`.

Event types and payloads must exist in `contracts/events/`. Consumers must be idempotent, preserve tenant context, acknowledge only completed work, and never treat the event bus as a cross-service database. Run `go test ./eventbus` from `platform/` after changes.
