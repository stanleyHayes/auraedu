# Admissions Service

Tenant-isolated applicant progress, document references, human review, offers and acceptance for AuraEDU Growth.

The service never stores uploaded document bytes; it stores File Service UUID references. AI/service-account roles cannot make admission decisions or issue offers.

## Selectable persistence (AURA-9.12)

`DATABASE_DRIVER` defaults to `postgres`. Set it to `mongodb`, provide
`MONGODB_URI`, and optionally set `MONGODB_DATABASE` (defaults to `admissions-service`).
Both server and worker select the same adapter and initialize its indexes before
processing requests. Each process uses a bounded two-connection pool.

The Mongo adapter scopes all request operations to a tenant and stores lifecycle
events inside their aggregate. Workers lease these events and acknowledge them
only after publication; redelivery keeps the same event ID.

MongoDB must be a replica set (including Atlas Free): cross-document operations
use transactions. This includes catalogue validation and admission creation. A standalone MongoDB server is not supported for those operations.

Run `go test ./internal/adapters/mongo` from this service to exercise a real
MongoDB replica-set testcontainer, including tenant isolation, concurrent claims,
and rollback. Docker is required.
