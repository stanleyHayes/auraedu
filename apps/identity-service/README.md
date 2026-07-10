# identity-service

Auth, users, roles, sessions (EP-04, L1).

## Responsibilities

- Authenticate users with email + password (argon2id).
- Issue signed JWT access tokens and rotating refresh tokens.
- Manage users, roles and permissions per tenant.
- Password reset and invite flows (tokenised; emit `notification.requested.v1`).
- Emit `user.role_changed.v1` domain events.

## JWT claims

Access tokens carry:

```json
{
  "sub": "<user_id>",
  "tenant_id": "<tenant_code>",
  "user_id": "<user_id>",
  "role": "<role>",
  "permissions": ["..."],
  "features_hash": "",
  "iat": 0,
  "exp": 0
}
```

`features_hash` is populated/enriched by the gateway from the Tenant Service snapshot.

## Environment

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8081` | HTTP port |
| `DATABASE_URL` | — | Postgres connection string; omit to use in-memory store |
| `JWT_SIGNING_KEY` | `dev-insecure-signing-key-change-me` | HS256 signing key |
| `JWT_ACCESS_TTL` | `15m` | Access-token lifetime |
| `JWT_REFRESH_TTL` | `168h` | Refresh-token lifetime |
| `REDIS_URL` | — | Redis connection string; omit for in-memory sessions |
| `SESSION_KEY_PREFIX` | `identity` | Redis key prefix |
| `NATS_URL` | — | NATS connection string; omit for no-op events |

## Local development

```bash
cd apps/identity-service
go test ./...
DATABASE_URL=postgres://... go run ./cmd/server
```

## Migrations

Service-local under `migrations/`. Run automatically at startup when `DATABASE_URL` is set.
