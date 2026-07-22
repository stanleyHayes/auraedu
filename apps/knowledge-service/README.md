# AuraEDU Knowledge Service

Tenant-isolated, review-gated source management and retrieval for AuraEDU Growth. Only approved, public, currently effective sources are returned by the authenticated internal search API. Draft, retired, expired, future-effective and internal sources fail closed.

The initial retrieval adapter uses PostgreSQL full-text ranking. The API exposes stable citation metadata so a later embedding or hybrid index can replace ranking without changing assistant contracts.

Approval and `knowledge.source_approved.v1` are committed atomically to a
tenant-isolated PostgreSQL outbox. The separately deployed `worker` command
publishes stable CloudEvent/JetStream identities with bounded retry, so an
approved source cannot become public while its lifecycle event is silently
lost. Event payloads contain citation identifiers and effective metadata only;
source content, ownership and reviewer notes remain private.
