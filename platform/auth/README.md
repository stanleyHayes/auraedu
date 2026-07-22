# `platform/auth`

Shared actor, JWT and authorization-registry primitives for Go services.

- `Sign` and `Verify` handle the internal HMAC JWT claims format and reject expired or tampered tokens.
- `Actor` represents the authenticated user, tenant, role and permission set; `WithActor` and `ActorFromContext` carry it through a request.
- `KnownPermissions`, `KnownRoles`, and `RoleScope` expose the generated authorization registry.

`registry.gen.go` is generated from `contracts/permissions/permissions.yaml`; change the contract and run `make contracts` instead of editing it. Services must still validate tenant access and resource scope—possessing a syntactically valid token is not sufficient authorization.

Run `go test ./auth` from `platform/` after changes.
