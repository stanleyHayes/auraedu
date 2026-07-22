# identity-service

Auth, users, roles, sessions (EP-04, L1).

## Responsibilities

- Authenticate users with email + password (argon2id).
- Keep platform identities globally unique by canonical email while allowing the same address in distinct school tenants.
- Require tenant-bound TOTP for school and platform administrators before issuing a session.
- Issue signed JWT access tokens and rotating refresh tokens.
- Consume refresh tokens atomically and revoke sessions after authorization or account-status changes.
- Manage users, roles and permissions per tenant.
- Password reset and invite flows with hashed, expiring, single-use credentials and private transactional delivery.
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
| `JWT_SIGNING_KEY` | required | HS256 signing key; server and worker fail startup when absent |
| `JWT_ACCESS_TTL` | `15m` | Access-token lifetime |
| `JWT_REFRESH_TTL` | `168h` | Refresh-token lifetime |
| `REDIS_URL` | — | Redis connection string; omit for in-memory sessions |
| `SESSION_KEY_PREFIX` | `identity` | Redis key prefix |
| `NATS_URL` | — | NATS connection string; omit for no-op events |
| `SERVICE_NOTIFICATION_URL` | — | Private Notification Service origin used for invite/reset delivery |
| `INTERNAL_SERVICE_TOKEN` | required in production | Shared credential for private notification and tenant activation calls |
| `MFA_ENCRYPTION_KEY` | required in production | Identity-only secret used to encrypt TOTP seeds and sign short-lived MFA challenges; minimum 32 characters |
| `AUTH_CLEANUP_INTERVAL` | `1h` | Identity worker interval for bounded authentication-artifact cleanup |
| `AUTH_CLEANUP_BATCH_SIZE` | `1000` | Maximum families or rows removed per artifact class per transaction; allowed range `1..10000` |
| `AUTH_REFRESH_RETENTION_AFTER_EXPIRY` | `24h` | How long a fully expired refresh family remains replay-detectable before deletion |
| `AUTH_PASSWORD_RESET_RETENTION` | `720h` (30 days) | Expired password-reset record retention |
| `AUTH_INVITE_RETENTION` | `2160h` (90 days) | Invite retention after the later of expiry or use |
| `AUTH_PUBLISHED_OUTBOX_RETENTION` | `720h` (30 days) | Successfully published Identity outbox retention; pending events are never cleaned |

## First-administrator invitation

Reviewed school onboarding creates an invitation in Identity and delivers a link through the private Notification Service. The browser posts the one-time token only to `POST /api/v1/public/invites/{token}/accept`; no existing session or active tenant is required. The Gateway rate-limits this canonical route without retaining the token in Redis or access logs. Successful acceptance creates the school administrator and activates the onboarding tenant before the web app selects that tenant for sign-in.

Invite acceptance commits token use, exact user matching and first-credential installation in one database transaction. A pre-created credentialless account can receive its first credential, but Identity never replaces an existing credential: a retry must prove the same password, role, permissions, active status, tenant and email before tenant activation resumes. A credential-write failure rolls back token use and leaves the invitation retryable. Issuing a replacement invitation records the older unused token as revoked, distinct from successful use, so a superseded link can never use the activation-retry path. Used invitations remain resumable only until their original expiry so a temporary Tenant Service activation failure does not strand the first administrator.

## Password recovery

Password recovery is tenant-bound even when two schools use the same email address. The Gateway requires a resolved canonical tenant for both recovery routes, Identity scopes the account lookup and one-time token to that tenant, and the public response remains non-enumerating. Reset links place the credential in the URL fragment so it never reaches the web server, reverse proxy or access logs. Identity enforces 12–256 Unicode characters for every newly assigned password, including administrative creation, invite acceptance and recovery. Reset-token consumption, credential replacement and complete refresh-session revocation are one database transaction; any failed credential or revocation write leaves the token unused and retryable.

Only the newest password-reset credential for an account remains active. Issuing a replacement revokes every older unused token, and a successful reset revokes any sibling token retained from an earlier deployment before the password change commits. Superseded, used, expired, cross-tenant and sibling tokens all fail with the same non-enumerating response.

School identities remain unique inside their tenant, so different schools may intentionally use the same email address. Platform identities have no tenant and are instead protected by a canonical-email partial unique index; migration refuses to continue if historical duplicates need operator reconciliation. This prevents privileged login from resolving an arbitrary platform account.

## Privileged multi-factor authentication

Production login for `school_admin`, legacy `super_admin` and `platform_super_admin` accounts is a two-step TOTP flow. Password verification returns a five-minute challenge instead of access or refresh tokens. First login supplies a one-time authenticator setup key; later logins request only the current six-digit code. Challenges are signed with a key derived separately from the shared JWT signing key and remain bound to the user, role and canonical tenant.

TOTP seeds are encrypted with AES-256-GCM using user and tenant identity as authenticated data. The plaintext seed is never logged, stored in a URL or returned after enrolment. The database advances the accepted RFC 6238 counter atomically, so concurrent or repeated use of a code fails. Replaying a setup challenge cannot replace an existing enrolment, and successful enrolment revokes existing refresh sessions.

Refresh rotation is a transactional session-family operation: the predecessor is revoked and its successor inserted together. Concurrent use of one token can create only one successor; reuse of a spent or expired family member revokes every descendant. Logout accepts any known family member, including a rotated predecessor, and family-scoped database locking serializes logout against rotation. Role, permission, locked/inactive status and password changes revoke every refresh session for that user. The MFA migration also revokes privileged sessions created before the assurance boundary existed, forcing a fresh password-and-TOTP login after rollout.

Production refresh requires both the authoritative PostgreSQL family member and a matching tenant/user session in Redis. A missing or identity-drifted Redis record revokes the complete database family and returns an unauthorized response; a Redis availability failure returns service unavailable without consuming the valid database token, so the client can retry after recovery. Successful rotation removes the predecessor cache key and writes the successor with the same refresh lifetime. PostgreSQL remains the revocation authority, so stale cache entries after administrative family/user revocation cannot make a token usable.

Role/permission changes commit their authorization event and session revocation with the user mutation. Locking or inactivating an account likewise updates status and revokes every refresh session in one PostgreSQL transaction; if revocation cannot commit, the status mutation rolls back. Refresh still re-checks current account status before issuing new claims as a defense in depth.

## Authentication artifact retention

The deployed worker performs retention cleanup at startup and then on a configurable interval. A refresh family is eligible only when its newest member has expired and the additional replay-detection window has elapsed; individual spent predecessors are never removed while a descendant may still be active. Each transaction removes at most the configured number of families and the same number of reset, invite and outbox rows through cleanup-specific indexes; an eligible family is removed atomically even when it contains multiple tokens. Password resets are kept for 30 days after the latest of expiry, use or revocation; invites are kept for 90 days on the same basis; and successfully published outbox records are kept for 30 days. Pending outbox records and processed-event idempotency records are outside this cleanup boundary. Invalid intervals, retention periods or batch sizes fail worker startup, while a transient periodic cleanup failure is measured, logged and retried at the next interval. Compose and Render pin the reviewed values explicitly, and the runtime configuration gate rejects drift between them.

## Local development

```bash
cd apps/identity-service
go test ./...
DATABASE_URL=postgres://... go run ./cmd/server
```

## Migrations

Service-local under `migrations/`. Run automatically at startup when `DATABASE_URL` is set.
