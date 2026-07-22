# `platform/tenancy`

Tenant-context enforcement shared by every Go service.

`FromRequest` derives a `TenantContext` from the verified bearer token and trusted gateway headers. `Middleware` attaches that context and can reject missing tenants. Context helpers propagate tenant, request and actor identity; `CacheKey` and `FilePath` namespace external storage; `ValidateAccess` prevents cross-tenant actor access.

The package also owns AuraEDU's tenant-aware `CloudEvent` envelope. Every persisted row, cache entry, object key, job, log correlation and event must retain tenant identity. Only the explicitly scoped platform-super-admin path may cross tenant boundaries. Run `go test ./tenancy` from `platform/` after changes.
