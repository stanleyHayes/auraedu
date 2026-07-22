# ADR 0002: Extract Growth content governance into content-service

- Status: Accepted
- Date: 2026-07-21
- Owners: Product, Architecture, L2 Content, L3 AI, L4 Frontend
- Decision scope: AuraEDU Growth AI Content Studio

## Context

ADR 0001 kept the first Growth slice deliberately small and required a new decision before
extracting another bounded context. The Growth MVP nevertheless requires generated content
drafts, institution brand rules, version history, independent review, expiry, and a strict
default prohibition on autonomous publishing. Campaigns own audience, budget, attribution,
schedule, and campaign lifecycle; they do not own creative evidence or its approval history.

Putting mutable content bodies and review evidence into `campaign-service` would couple two
different lifecycles, prevent reuse outside campaigns, and make it harder to prove that a
campaign approval did not silently approve a later creative revision.

## Decision

Create an independently deployable `content-service` with its own tenant-isolated database.
It owns:

- institution content policy and brand rules;
- generated and human-refined content drafts;
- immutable version history and generation provenance;
- deterministic brand-compliance findings;
- independent submit, approve, reject, and expiry transitions;
- privacy-safe content lifecycle events.

Campaign IDs are optional references, never foreign database keys. Content approval applies
to exactly one immutable version. Editing approved or rejected work creates a new draft
version and invalidates the prior approval. There is no automatic publishing endpoint in the
MVP. A later publishing integration must require `content.publish`, an approved unexpired
version, a separate delivery contract, and another architecture/security review.

Generation is provider-agnostic behind an application port. The service sends only the
tenant-approved brand policy, explicit brief, and supplied fact set to the configured
generator. The returned copy is always a draft. Provider failure must fail closed and must not
create empty or fabricated content.

## Consequences

- `content-service` can scale and retain content independently of campaign tracking.
- Campaign approval never implies content approval and vice versa.
- Content bodies and briefs never appear in integration events, logs, or audit metadata.
- The Gateway enforces the `growth_content_ai` entitlement; the service independently
  enforces tenant scope, feature state, and `content.generate` / `content.review` permissions.
- Publication remains explicitly out of scope until a real channel adapter can prove delivery.

## Rejected alternatives

- Store creative copy in `campaign-service`: rejected because lifecycle and approval evidence
  diverge, and non-campaign content would have no owner.
- Generate copy only in the browser: rejected because it bypasses tenant policy, audit,
  permissions, provider governance, and version history.
- Treat campaign approval as creative approval: rejected because an approved campaign can
  contain a later or materially different draft.

## Verification

- OpenAPI and CloudEvent contracts generate compiling Go and TypeScript clients.
- Domain tests prove four-eyes review, immutable approved versions, rejection/revision,
  expiry, and no publish transition.
- PostgreSQL tests prove FORCE RLS, cross-tenant denial, append-only versions, and atomic
  state-change outbox writes.
- Gateway, service, and web tests prove feature/RBAC gates and review workflow boundaries.
