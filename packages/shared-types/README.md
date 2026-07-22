# `@auraedu/shared-types`

Contract-derived TypeScript and Go types for AuraEDU OpenAPI documents, CloudEvents schemas, permissions, roles, features, and gateway runtime models.

## Generated-code rule

Files below `src/generated/` and `gen/go/` are generated artifacts. Never hand-edit them. Change the source contract under `contracts/`, then run:

```sh
make contracts
```

Import the root namespaces (`OpenAPI`, `Events`, `Authorization`, and `Features`) or a documented package subpath. A contract change is complete only when generation, drift checks, build, tests, and affected producer/consumer checks pass.
