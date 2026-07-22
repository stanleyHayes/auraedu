# Chapter 35: Security

## Purpose

Protect students, guardians, educators, schools and AuraEDU operators by making identity, tenant isolation, authorization, data minimization and operational safety enforceable properties of the system. Security controls must fail closed and produce reviewable evidence.

---

## Scope

This chapter covers public and private network boundaries, authentication, sessions, service identity, RBAC, tenant and learner ownership, database isolation, secrets, logging, files, AI actions, dependencies, build provenance, deployment and incident response. Product-specific privacy and retention rules remain in their domain chapters but may not weaken these controls.

---

## Principles

- Deny by default at every trust boundary.
- Authenticate identity, then independently authorize tenant, role, permission, feature and resource ownership.
- Treat every client-provided identity, tenant, role and resource identifier as untrusted.
- Give people and workloads only the access needed for the current operation.
- Keep personal data out of logs, events, metrics, URLs and error details.
- Make high-impact AI output advisory or independently human approved.
- Prefer short-lived, rotatable credentials and auditable changes.
- Turn critical assumptions into CI gates rather than relying on conventions.

---

## Business Rules

- A school user may access only their active tenant and only resources permitted by their role and ownership relationship.
- Platform administrators must use explicit platform-scoped permissions; the platform role is not accepted as an implicit tenant membership.
- Platform identity email is globally unique after canonicalization. Tenantless identities may never rely on a nullable composite unique constraint or ambiguous first-row selection.
- School and platform administrators must complete tenant-bound TOTP after password verification before Identity issues a session.
- Teachers may access only assigned classes and learners. Parents may access only linked children. Students may access only their own learner record.
- Applicant access is restricted to the applicant's own drafts, documents, offers and decisions.
- AI cannot independently approve admissions, grades, fees, payments, publication, bulk actions, security changes or destructive operations.
- Invite, password-reset, refresh and provider secrets must not be persisted in notification bodies, logs or event payloads.
- Security exceptions require a named owner, expiry date, compensating control and accepted ADR or incident record.
- A vulnerability or suspected cross-tenant disclosure is a security incident, not ordinary product support.

---

## Technical Rules

### Network and deployment boundaries

- API Gateway, Web and Marketing are the only public web services in the Render Blueprint.
- Domain APIs and NATS are private services; background processors remain worker deployments.
- All 24 managed PostgreSQL instances and the Key Value store set `ipAllowList: []`; omission is prohibited because it permits public-IP ingress on Render.
- Services connect through Render private `connectionString` references. Public database URLs must not be injected into application deployments.
- The gateway is not the sole authorization layer. Every domain handler must enforce permission, tenant and ownership checks again.
- Redis-backed public abuse protection and Tenant Service resolution are required edge dependencies. Production startup fails without Redis; Tenant lookup outages return unavailable and never manufacture context from an unverified header.
- Live feature-entitlement lookups use a bounded shared client: four-second total timeout, safe tenant query encoding and a 1 MiB response ceiling. Missing, slow, non-successful, oversized or malformed Tenant Service responses deny production access instead of using registry defaults.
- Every Go HTTP entrypoint composes the shared request boundary before domain handlers. Standard request bodies are capped at 1 MiB; multipart uploads receive a 40 MiB outer ceiling and remain subject to stricter File Service validation. Declared oversized bodies return the canonical `413 payload_too_large` response with a request ID; chunked bodies remain bounded by `http.MaxBytesReader`.
- Internal Go HTTP clients decode JSON through the shared 1 MiB response ceiling. Oversized, malformed, multi-value or trailing-data responses fail closed as dependency errors; a private network and service credential do not make an unbounded dependency response safe.
- Python AI learner-scope and feature-entitlement clients apply the same 1 MiB plus one-byte overflow check before JSON decoding. A truncated prefix is never accepted as a complete authoritative response, and production feature checks deny when the response exceeds the bound.
- Recommendation, Prediction and Career Guidance apply a streaming ASGI request ceiling of 1 MiB before FastAPI parses a model. Declared and chunked overflow both return the same JSON `413 payload_too_large`; Prometheus remains outside the limit so rejected traffic is measured.
- Production browser access is limited to explicit HTTPS AuraEDU origins, owned `*.auraedu.com` school subdomains and exact custom hostnames activated through AURA-9.5. Custom domains require a one-time hashed DNS TXT challenge plus separate platform attestation of provider TLS; pending or merely verified records never resolve publicly. Wildcard-all, first-label lookalikes, plaintext, custom ports and malformed origins fail closed.
- Gateway API responses are non-cacheable, non-embeddable and non-sniffable by default, disclose no referrer, disable unused device capabilities and use a `default-src 'none'` CSP. Production also emits HSTS.
- Render production derives public rate-limit identity from Cloudflare's single-address `CF-Connecting-IP`, never the client-controlled first `X-Forwarded-For` value. Missing trusted evidence falls back to the direct peer.
- Production containers run as non-root users and all base, CI service and infrastructure images use immutable digests.

### Authentication and sessions

- Identity is the only issuer of access and refresh tokens. Production startup fails without a strong signing key and Redis-backed session storage.
- Refresh requires a matching tenant/user Redis session before PostgreSQL rotation. Missing or identity-drifted cache state revokes the authoritative database family; Redis unavailability returns retryable service-unavailable without consuming the token. Redis never overrides a PostgreSQL revocation.
- Access tokens are short lived. Refresh tokens rotate, revoke their predecessor and are bound to a durable server session.
- Refresh tokens form server-owned session families. Rotation revokes the predecessor and inserts its successor in one transaction; concurrent replay produces at most one successor, and reuse of any spent or expired member revokes every descendant. A find-then-revoke sequence is prohibited.
- Logout and administrative revocation invalidate the complete refresh family. Family-scoped transaction locking serializes rotation against revocation so a child cannot survive through a snapshot or insertion race.
- Spent refresh-family members remain replay-detectable until every descendant has expired, plus a 24-hour retention window. Cleanup must evaluate the family maximum expiry and may never delete individual predecessors from a potentially live family.
- Authentication cleanup uses indexed oldest-first candidates and bounded transactions. Its configured batch may limit families, resets, invites and published events, but every selected refresh family is deleted atomically.
- Role, permission, locked/inactive status and password changes revoke all refresh sessions. Refresh re-checks active account status before issuing current-role claims.
- Authorization/status mutation and refresh-session revocation are one database transaction. An account must never appear locked or inactive while its revocation write silently failed; the entire mutation rolls back.
- Logout revokes the active session; administrative session revocation is supported.
- Authentication responses and logs must not reveal whether an unrelated tenant, account or invitation exists.
- Every newly assigned password is 12–256 Unicode characters. Frontend hints and OpenAPI constraints mirror, but never replace, Identity's authoritative validation.
- Production privileged login returns a five-minute, purpose-specific MFA challenge after password verification. The challenge is signed with an Identity-only derived key and binds user, role and canonical tenant; it is never an access token.
- TOTP seeds use 160 bits of cryptographic randomness and are encrypted at rest with AES-256-GCM. User and tenant identity are authenticated data. The plaintext seed may be shown only during first enrolment and must not enter URLs, logs, events or browser storage.
- Enrolment is insert-once, successful enrolment revokes existing refresh sessions, and every accepted RFC 6238 counter advances atomically. Setup or code replay must fail, including under concurrent requests.
- Mobile session material uses encrypted platform storage, validates its schema on restoration and clears on identity or tenant drift.

### Authorization and tenancy

- `contracts/permissions/permissions.yaml` is the source of truth for roles and permission keys; generated registries reject unknown values.
- The server derives actor identity and effective tenant from authenticated context. Body, query and header values cannot elevate them.
- Permission checks, feature checks and ownership checks are separate decisions. Passing one never implies the others.
- Tenant-owned PostgreSQL tables enable and force RLS. Repositories set tenant context inside the transaction before tenant queries.
- Internal cross-service reads propagate the actor required for scope enforcement and authenticate with the shared internal service token.
- Service-to-service failure on an authoritative ownership check returns unavailable/denied; it must not broaden access.
- Deterministic UPSHS/Aboom tenant and feature snapshots are local development fixtures only and must remain empty in production.

### Data, events and observability

- Application logs use the shared redacting logger. Raw request bodies, query strings, tokens, email addresses, phone numbers and provider secrets are prohibited log arguments. Gateway access logs retain bounded `CF-Ray` correlation instead of URL parameters.
- Metrics labels use allowlisted low-cardinality values and never include tenant, learner, recipient or free-form identifiers.
- CloudEvents carry stable IDs and privacy-safe identifiers. Transactional outboxes prevent state changes from being committed without their required event.
- Identity retains password-reset records for 30 days after the latest of expiry, use or revocation; invitations for 90 days on the same basis; and published outbox records for 30 days. Pending outbox and durable idempotency records are never removed by authentication cleanup.
- Audit records are append-only and tenant isolated. High-impact lifecycle and AI-action decisions retain actor, policy and outcome evidence.
- Database backups are encrypted, isolated and restored into a new target before cutover; Chapter 32 defines recovery controls.

### Files and input

- Validate request size, content type, filename, extension and magic/content evidence at the trusted boundary.
- Download authorization is re-evaluated; possession of a file identifier or URL is not authorization.
- Uploaded admissions documents must match the tenant, owner and accepted file status before attachment.
- SQL uses parameterized queries. Shell commands must not concatenate untrusted input.
- Redirect targets and public API origins are parsed and allowlisted; credentials, fragments and unsafe schemes fail closed.

### Secrets and supply chain

- Runtime secrets are injected by the deployment provider and never committed in `.env` files.
- Private-service credentials are required in production and may not fall back to development defaults.
- Remote GitHub Actions use full commit SHAs. Tool versions, container bases, service images and testcontainer images are immutable.
- Dependency installation uses lockfiles. High-severity JavaScript and Python advisories and Go vulnerability checks run in CI.
- Generated authorization and contract artifacts are regenerated from reviewed sources; generated files are never hand edited.

---

## Architecture

```text
internet
  |-- Marketing (public content and bounded onboarding request)
  |-- Web (browser UI; no direct database access)
  `-- API Gateway (authentication edge, routing, rate limits)
           |
           | private Render network + authenticated service calls
           v
      domain private services ----> service-owned PostgreSQL (public ingress denied)
           |                                  |
           |                                  `-> FORCE RLS + transactional outbox
           `-------------------------------> NATS private JetStream

human / AI request
  -> authenticated actor
  -> tenant boundary
  -> feature policy
  -> permission policy
  -> learner/resource ownership
  -> validated mutation
  -> immutable audit/outbox evidence
```

The gateway reduces public attack surface but is not a trusted shortcut. Domain services remain the final policy enforcement point. Databases enforce tenant isolation beneath repository code, and event consumers repeat tenant and schema validation before applying state.

First-administrator onboarding uses a deliberately narrow public exception. A reviewed request creates a hashed, expiring, single-use invite; Notification Service builds the acceptance link only from the production-validated HTTPS app origin, places the credential in a URL fragment that is not sent to the web server or proxy, and erases its full content after provider handoff. The non-secret tenant query may bootstrap the shared application hostname only after strict single-DNS-label canonicalization. The public Gateway route accepts only the token activation operation, needs no pre-existing tenant session, rate-limits by canonical route and client address, and never writes the credential-bearing path to Redis or access logs. Identity remains the authority for token validity, role grant, replay-safe user creation and tenant activation.

Identity commits invite use, exact account matching, user creation and first-credential installation in one transaction. A credentialless pre-created user may receive one credential; an already credentialed user must prove the submitted password, and no invite path may replace that credential. A failed user or credential write leaves the invite unused. Successful first-administrator acceptance may be replayed with the same credential only until the original expiry so a temporary Tenant activation outage is recoverable. Replacement invitations revoke prior unused tokens with a separate `revoked_at` state; revocation is never interpreted as successful use or as permission to resume activation.

Platform identities use `tenant_id = NULL`, so the school-scoped `(tenant_id, email)` uniqueness rule cannot protect them under ordinary PostgreSQL NULL semantics. Identity therefore enforces a separate canonical-email partial unique index for tenantless users. The migration fails with an explicit reconciliation error if historical duplicates exist; it never chooses, merges or deletes a privileged account automatically. The same canonical email may still exist once in each independent school tenant.

Password recovery follows the same server-invisible credential rule but always requires a canonical tenant boundary. Identity resolves email addresses inside that tenant, binds the hashed one-time token to it and refuses cross-tenant consumption, including when two schools have the same email address. Public responses do not disclose whether an account exists. Token consumption, credential replacement and complete refresh-session revocation commit in one transaction; failure rolls all three back. The web client also clears its local authentication cookies before returning to sign-in.

Issuing a new password-reset credential revokes every older unused token for that tenant account. Successful consumption also revokes any sibling credential retained from an earlier deployment before the password and session mutations commit. This prevents an older mailbox link from undoing a completed recovery while preserving the same non-enumerating failure response for expired, used, revoked and cross-tenant tokens.

Privileged authentication adds a separate possession factor without turning the browser into a credential store. The password-authenticated response carries a short-lived challenge in React action state; the portal neither redirects with it nor persists it. First-time administrators manually add the one-time seed to a compatible authenticator and prove possession immediately. Identity stores only the AES-GCM ciphertext and the last accepted counter. Later verification decrypts inside Identity, validates a plus-or-minus 30-second clock window and commits a strictly greater counter before issuing access and rotating refresh tokens.

---

## Best Practices

- Write negative tests first: missing token, wrong tenant, wrong role, wrong learner, disabled feature and unavailable ownership dependency.
- Test with two real tenants against PostgreSQL so memory adapters cannot conceal RLS failures.
- Keep public errors stable and non-sensitive while retaining redacted correlation evidence internally.
- Canonicalize dynamic paths before logging or rate-limit persistence; resource IDs, reset tokens and invite tokens do not belong in edge metadata.
- Rotate credentials after suspected disclosure and verify old credentials fail.
- Review new routes against OpenAPI, gateway inventory, permissions and feature registries in one change set.
- Use a separate human reviewer for policy approval and the exact downstream permission for controlled actions.
- Re-run the Render network boundary gate whenever a service, database or cache is added.

---

## Examples

Run the local executable security gates:

```bash
bash tools/ci/security-static-scan.sh
bash tools/ci/check-render-network-boundaries.sh
bash tools/ci/run-authz-security-tests.sh
bash tools/ci/run-injection-upload-security-tests.sh
bash tools/ci/check-tenant-rls.sh
```

An assigned teacher requesting a learner report passes authentication and `reports.read`, then the Report Service asks Student Service for the authoritative assigned-learner set. If that dependency is unavailable or the learner is outside the set, the request fails closed.

An AI lead-assignment proposal is inert until a different authorized human approves the exact fingerprinted payload. Unknown or high-impact tools cannot be proposed, and approval cannot expand the stored action.

---

## Anti-patterns

- Trusting a tenant header because the gateway supplied it.
- Checking only a role name while ignoring permissions and ownership.
- Filtering by tenant in application code without FORCE RLS beneath it.
- Returning an empty list when an authorization dependency failed.
- Logging an entire webhook, request, event or provider error body.
- Using a persistent disk as the only backup.
- Exposing PostgreSQL publicly because credentials are required.
- Pinning a container tag without its digest or a GitHub Action by mutable version tag.
- Allowing AI-generated content or actions to bypass the same permissions required of a human.

---

## Checklist

- [ ] Public/private network inventory is unchanged or explicitly reviewed.
- [ ] Authentication, tenant, feature, permission and ownership controls are identified separately.
- [ ] Unknown roles, permissions, events and feature keys fail closed.
- [ ] Two-tenant positive and negative tests exercise the real database policy.
- [ ] Personal data is absent from logs, metrics, events, URLs and error responses.
- [ ] Inputs, files, redirects and outbound destinations are bounded and validated.
- [ ] Production entitlement checks fail closed through a bounded Tenant Service client; stalled or oversized responses cannot pin handlers or grant registry defaults.
- [ ] Every production Go server composes the shared request boundary; known and chunked oversized bodies are bounded, multipart allowance does not weaken file validation, and CI rejects an uncovered entrypoint.
- [ ] Internal service clients use the shared bounded JSON decoder; CI rejects direct response-body decoding in client adapters.
- [ ] Python AI dependency reads detect overflow rather than silently parsing a truncated prefix; tests prove learner scope raises and production entitlement resolution fails closed.
- [ ] Every Python AI API installs the streaming request-body middleware inside metrics; declared-length and chunked overflow tests return canonical `413` without invoking the route.
- [ ] Secrets have no committed or development fallback path in production.
- [ ] Every privileged role proves TOTP enrolment, tenant binding and setup/code replay denial before session issue.
- [ ] Concurrent refresh replay yields exactly one successor, reuse revokes its complete family, logout wins against concurrent rotation, and authorization/status changes invalidate all prior refresh sessions transactionally.
- [ ] Missing or identity-drifted Redis refresh state revokes the database family; a Redis outage returns unavailable without consuming a valid token, and successful rotation replaces the cache key.
- [ ] Forced session-revocation failure rolls back role, permission and locked/inactive status mutations.
- [ ] New-password length is enforced by Identity and the contract; forced credential/session failure leaves a reset token unused, while success consumes it exactly once.
- [ ] A new reset credential revokes older unused tokens, and successful recovery revokes any legacy sibling before the password mutation commits.
- [ ] Invite acceptance atomically commits token use, exact user state and first credential; forced credential failure leaves the token unused, existing credentials are never replaced, and replacement invites revoke every older unused token.
- [ ] Canonical platform email uniqueness is database-enforced; same-email identities remain valid across separate school tenants, and migration refuses ambiguous privileged duplicates.
- [ ] Retention cleanup preserves every potentially live refresh family and every pending outbox event; invalid cleanup policy fails startup.
- [ ] Dependencies and automation references are locked and immutable.
- [ ] High-impact actions retain independent human approval and audit evidence.
- [ ] Runbooks, alerts and recovery implications are updated.

---

## Definition of Done

- Contract, gateway route, permission and feature registries agree.
- AuthN/Z, injection/upload, tenant-RLS, static secret/PII and dependency gates pass.
- Direct API tests prove wrong-tenant, wrong-role, wrong-owner and dependency-failure denial.
- Production runtime fails closed when required secrets or authoritative dependencies are absent.
- Render topology proves only the intended public services and denies datastore public ingress.
- Container/runtime identity and supply-chain pins pass CI.
- Any provider-specific control is verified in the deployed environment with retained evidence.

Repository evidence does not prove deployed secret rotation, provider IAM, paging delivery, marketplace/store controls or live Render settings that exist outside the Blueprint. Those require environment evidence before production approval.

---

## References

- [Permissions registry](../../../contracts/permissions/permissions.yaml)
- [Render Blueprint](../../../render.yaml)
- [Disaster Recovery](../04-operations/32-disaster-recovery.md)
- [AI coding-agent controls](../07-governance/44-ai-coding-agent-instructions.md)
- [Render Blueprint IP allow lists](https://render.com/docs/blueprint-spec)
- [Agent execution plan](../../../agent_plan.md)
