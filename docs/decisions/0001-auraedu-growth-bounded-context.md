# ADR 0001: AuraEDU Growth bounded context and first delivery slice

- Status: Accepted
- Date: 2026-07-18
- Owners: Product, Architecture, L0 Platform, L2 CRM, L3 AI, L4 Frontend
- Decision scope: AuraEDU Growth

## Context

AuraEDU Growth adds recruitment, marketing, admissions conversion and institution-controlled AI to the existing education operating system. It depends on AuraEDU tenancy, identity, feature flags, billing, notifications, audit and analytics. A separate repository or duplicate platform would create conflicting identity and governance boundaries.

## Decision

Growth is a first-class product domain in the existing monorepo. New capabilities use independently deployable bounded contexts only when the boundary has its own data and lifecycle. Existing AuraEDU services remain authoritative for identity, tenant configuration, admissions, payments, notifications, analytics and audit.

The first vertical slice is:

```text
tenant programme page -> consented enquiry -> deduplicated CRM lead
-> admissions officer lead list -> interaction timeline -> feedback event
```

Delivery is contracts-first. The initial public write surface is restricted to lead capture; staff reads and mutations require authenticated tenant-scoped permissions. All high-impact AI and campaign actions remain approval-gated.

## Service boundaries

- `crm-service`: leads, stages, assignments, consent, interactions and tasks.
- `campaign-service`: campaigns, audiences, budgets, attribution and approvals.
- `knowledge-service`: approved tenant sources, retrieval and citations.
- `ai-orchestrator-service`: agent policy, tools, approvals, model routing and cost.
- Existing services: admissions, notification, analytics, audit, billing, tenant and identity remain systems of record.

## Consequences

- No separate Growth repository or duplicate authentication stack.
- No custom ML scoring in the MVP; scoring begins as explainable rules.
- Public lead capture must be rate-limited, idempotent and tenant-resolved at the gateway.
- Email/phone deduplication is tenant-local and uses normalized hashes where practical.
- Growth features are disabled by default and activated through plan entitlement plus tenant override.
- Later service extraction requires a new ADR and published contracts.

## Rejected alternatives

- A separate Growth platform: rejected because it duplicates shared controls.
- Dozens of services immediately: rejected because the operational boundary is not yet proven.
- Autonomous campaign publishing or spend: rejected because human approval is a product invariant.

## Verification

- Contract lint and generated clients pass.
- Tenant-isolation, permission, consent, deduplication and idempotency tests are mandatory for the CRM producer.
- The first end-to-end test must exercise enquiry through staff-visible lead history.
