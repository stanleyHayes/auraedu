# `platform/flags`

Server-side feature gates for HTTP services and workers.

`TenantServiceClient` reads live tenant overrides. Service entrypoints use `NewRuntimeGate`: development may fall back to a registry-derived `StaticSnapshot`, while production fails closed whenever Tenant Service is missing, unavailable, non-200, or malformed. `RequireEnabled` returns the shared feature-disabled error for transport adapters; the fallback wrappers emit only one bounded operational warning per process.

Missing and unknown features are disabled. A feature flag never grants authorization, and worker consumers must gate processing as well as HTTP routes. The source registry is `contracts/features/features.yaml`. Run `go test ./flags` from `platform/` after changes.

`registry.gen.go` is generated from that contract and exposes typed `Feature*` constants, `KnownFeatures`, and `IsKnownFeature`. Tenant Service derives its runtime catalogue from this generated registry; never add a key to either service by hand. Change the contract and run `make contracts` instead.
