# Chapter 46: Release Process

## Purpose

Define the evidence path from a reviewed change to a production release. AuraEDU is production ready only when code, contracts, tenant isolation, security, performance, recovery, provider delivery, deployment and client-distribution evidence are all current and approved.

---

## Scope

All backend services, workers, contracts, migrations, web applications, tenant websites, mobile binaries, infrastructure, scheduled recovery jobs and external delivery providers.

---

## Principles

- A passing build proves buildability, not production readiness.
- Evidence must cover the same environment and boundary as the claim.
- Planned, local-only and deployed states remain visibly distinct.
- Release approval is human-accountable and machine-checkable.
- Evidence contains no credentials, access tokens or unnecessary personal data.

---

## Business Rules

- Product owns release scope and customer-facing change communication.
- Engineering owners attest their service and contract evidence.
- Security and Privacy approve unresolved risk and access to sensitive recovery data.
- SRE owns deployment, observability, scaling, rollback and recovery evidence.
- Mobile Release owns signed-binary, store and OTA-channel evidence.
- No owner may waive a failed gate by changing its status without a documented exception and expiry.

---

## Technical Rules

1. Pull requests pass the path-routed CI matrix and generated-contract drift checks.
2. Migrations are additive, tenant RLS is exercised on real PostgreSQL, and event consumers are replay safe.
3. Production images are pinned, non-root, health-gated and configured without development fallbacks.
4. Staging proves authenticated domain workflows, two-tenant isolation, load thresholds and complete telemetry flow.
5. Provider-dependent features prove actual provider acceptance in the deployed environment.
6. Mobile releases use signed binaries, isolated EAS channels and recorded store submissions.
7. Recovery approval requires provider configuration plus a hosted restore and cutover exercise; a local archive test is insufficient.
8. Retained evidence is listed in `release/evidence/manifest.json` and verified by SHA-256.

---

## Architecture

The live status table in `agent_plan.md` is the delivery ledger. Every `In progress` row must have one matching `pending` item in the release-evidence manifest. Verified evidence records live below `release/evidence/records/<AURA-ID>/`; the verifier bounds paths to that directory, checks non-empty files and hashes, and requires a timestamp and named approver before accepting `verified` status.

`make release-evidence-validate` checks structure, hashes and ledger parity. `make release-readiness` applies the same checks and fails until both the manifest and ledger contain no pending release items.

---

## Best Practices

- Prefer machine-readable JSON results, provider exports and immutable build identifiers.
- Record the commit, environment, start/end time, actor and sanitized command for each external run.
- Capture populated, empty, failure and recovery states for visual workflow evidence.
- Hash evidence immediately after collection and review it in the same change as the ledger transition.
- Re-run environment-sensitive evidence after material infrastructure or configuration changes.

---

## Examples

- A load-test record contains the deployed base URL hostname, commit SHA, scenario hash, unique tenant count, request totals, latency percentiles, error rate and threshold result.
- A provider-delivery record contains provider message ID, channel, acceptance state and timestamps but excludes recipient content and credentials.
- A browser evidence set contains desktop and mobile captures plus route, role, tenant, viewport, commit SHA and console/accessibility results.
- A recovery record contains source and restored fingerprints, RPO/RTO measurements, bucket retention inspection and signed cutover/rollback decisions.

---

## Anti-patterns

- Marking a deployed story Done because local unit tests pass.
- Treating a screenshot as proof of tenant isolation, provider acceptance or recovery integrity.
- Copying secrets or student records into a release artifact.
- Accepting a provider dashboard configuration without exercising delivery.
- Deleting a pending manifest item without updating and proving its ledger outcome.

---

## Checklist

- [ ] CI, contracts, migrations, security and tenant-isolation gates pass.
- [ ] Staging workflows and performance thresholds pass against the release commit.
- [ ] Metrics, traces, logs, alerts and paging reach their deployed destinations.
- [ ] Provider-backed email, push, payment or messaging paths used by the release are proven.
- [ ] Web and mobile visual/accessibility evidence covers the changed journeys.
- [ ] Signed mobile builds, store submissions and OTA channels are recorded when applicable.
- [ ] Backup retention, restore and cutover evidence meets Chapter 32.
- [ ] Rollback owner, decision point and commands are current.
- [ ] `make release-readiness` succeeds.

---

## Definition of Done

- Every requirement in the release scope has environment-matched evidence.
- The evidence manifest and live ledger agree.
- Every retained artifact is sanitized, non-empty and hash verified.
- Required owners have approved the evidence.
- The production-readiness command exits successfully with zero pending items.
- Post-deploy health and core workflow checks pass before the release is announced.

---

## References

- [Release evidence instructions](../../../release/evidence/README.md)
- [Release evidence manifest](../../../release/evidence/manifest.json)
- [Release evidence verifier](../../../tools/release/verify-readiness.mjs)
- [Disaster recovery](../04-operations/32-disaster-recovery.md)
- [Monitoring](../04-operations/31-monitoring.md)
- [Agent execution plan](../../../agent_plan.md)
- [Design system](../../../DESIGN_SYSTEM.md)
