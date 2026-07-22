# Chapter 05: Non-Functional Requirements

## Purpose

Set measurable quality attributes for availability, performance, security, accessibility, localisation, offline use, scalability, recovery and maintainability.

---

## Scope

Platform-wide service levels and the evidence required before targets are claimed.

---

## Principles

- Targets are budgets that shape architecture and operations.
- No reliability claim is published without measured evidence.
- Graceful degradation is designed, not improvised.
- Accessibility, privacy and recovery are release concerns.

---

## Business Rules

- The production availability objective is 99.95 percent once formally adopted by an SLA ADR.
- Schools receive clear maintenance, incident and recovery communication.
- Critical teacher workflows should tolerate intermittent connectivity through queued or offline-capable designs where supported.
- Localisation includes language, timezone, date, currency and academic-calendar behaviour.

---

## Technical Rules

- Define latency SLOs by endpoint class and measure p50, p95 and p99.
- Encrypt data in transit and at rest; redact sensitive data from logs.
- Meet WCAG 2.2 AA for web and equivalent mobile accessibility guidance.
- Set RPO and RTO per data class; prove restoration through exercises.
- Scale services horizontally without cross-tenant state in process memory.

---

## Architecture

Service-level indicators feed observability and release gates. Capacity, recovery and accessibility evidence are stored with runbooks and test results.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- Login and attendance have tighter latency budgets than asynchronous report generation.
- A notification provider outage queues work without blocking the school portal.
- A restore drill proves the documented RPO and RTO instead of assuming backups work.

---

## Anti-patterns

- Writing 99.95 percent on marketing pages before monitoring and incident policy exist.
- Treating offline as a universal promise without conflict rules.
- Using average latency to hide slow-tail behaviour.

---

## Checklist

- Is the target measurable?
- Is an owner and data source defined?
- Is degraded behaviour documented?
- Has recovery been exercised?
- Are accessibility and localisation verified?

---

## Definition of Done

- Every adopted SLO has an indicator, alert and owner.
- Security, accessibility and performance gates run in CI or staging.
- Recovery targets are tested on a documented schedule.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
