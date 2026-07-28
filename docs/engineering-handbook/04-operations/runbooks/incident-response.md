# Incident-response runbook

This is the skeleton every AuraEDU incident follows. It covers coordination, severity,
communication and the tenant-isolation-breach playbook. It deliberately does **not** cover
data restore — once an incident is declared and data loss or corruption is in scope, switch
to [disaster-recovery.md](disaster-recovery.md) and run both in parallel. Per-alert
diagnosis lives in [alerts.md](alerts.md); this runbook is about running the incident, not
fixing a specific alert.

## 1. Roles

Assign by name at declaration time. One person may hold two roles in a small incident;
the Incident Commander role is never shared.

| Role | Owns | Does not |
|---|---|---|
| **Incident Commander (IC)** | Severity call, timeline, decisions, delegation, declaring all-clear | Hands-on debugging (stays out of the terminal once the team is staffed) |
| **Comms Lead** | All internal and external messages (§6), status updates, school-facing and regulator drafts | Technical changes |
| **Ops Lead** | Executing the technical plan the IC approves | Changing severity or messaging unilaterally |
| **Scribe** (may be IC in SEV3/4) | Timestamped log: declarations, decisions, actions, evidence locations | — |

For data-loss incidents add the Recovery Lead, Data Owner and Security/Privacy approver
defined in [disaster-recovery.md](disaster-recovery.md) §1.

## 2. Severity ladder

Severity drives paging, comms cadence and who must be woken up. It maps onto the existing
Alertmanager routing in `infrastructure/observability/alertmanager.yml` and the alert rules
in `infrastructure/observability/rules/auraedu-alerts.yml` (labels `severity` × `team`).

| Level | Meaning | Trigger examples | Response |
|---|---|---|---|
| **SEV1** | Data breach, cross-tenant exposure, money integrity compromised, or platform-wide outage | Any confirmed/suspected tenant-isolation breach (§5); payment double-settlement or webhook forgery; gateway down for all schools | Page IC + team on-call immediately; comms to affected schools ≤ 1h; regulator clock may be running (§5.4) |
| **SEV2** | A core workflow is down for one or more schools; no data exposure | `AuraEDUServiceDown` for a core service (identity, gateway, fees, payment); `AuraEDUPaymentWebhookFailures` (critical, team `payments`); `AuraEDUHighHTTPErrorRate` (critical, team `platform`) | Page team on-call; IC assigned; school comms if user-visible > 30 min |
| **SEV3** | Degraded but working; user impact limited | `AuraEDUHighP95Latency`, `AuraEDUNotificationDeliveryFailures` (critical, team `communications` — messages delayed, not lost), `AuraEDUAIServiceLatency` | Work during business hours; comms only if a school asks |
| **SEV4** | No user impact; early-warning signal | `AuraEDUWorkerJobFailures`, `AuraEDUNotificationAPIFailures`, `AuraEDUSustainedFailedLogins` (warning, team `security` — watch, could escalate to SEV2/1 if targeted) | Ticket; fix in sprint |

Escalate severity on new information; never silently downgrade. `team="security"` alerts
route to the security receiver regardless of severity — treat every one as potential SEV1
until triaged.

## 3. Paging and receivers (placeholders)

The checked-in Alertmanager config uses local-only receivers (`local-observability`,
`local-critical`, `local-security`, `local-warning`) so development alerts stay on the
workstation. Before production launch, replace them with secret-backed receivers while
keeping the label routing and inhibition policy unchanged:

- PagerDuty integration key (critical severity): `<PAGERDUTY_CRITICAL_ROUTING_KEY>`
- PagerDuty integration key (security team): `<PAGERDUTY_SECURITY_ROUTING_KEY>`
- Warning receiver (Slack/email): `<WARNING_CHANNEL_WEBHOOK>`
- On-call rotation and escalation policy: `<PAGERDUTY_ESCALATION_POLICY_URL>`
- Status-page / school-comms channel: `<STATUS_PAGE_URL>`

Until these are wired, **alert delivery is a known launch blocker** (tracked under
AURA-8.1's remaining production evidence) and this runbook's paging steps are aspirational.

## 4. First 15 minutes

1. **Declare.** Someone says "this is an incident" in the incident channel, names the
   severity guess, and becomes or appoints the IC. Undeclared incidents do not get fixed
   faster; they get fixed twice.
2. **Staff the roles** (§1). Start the timestamped log now.
3. **Snapshot the blast radius.** Which tenants, which services, since when? Check the
   Golden Signals dashboard and `/api/v1/platform/health` (superadmin console
   `/superadmin/system-health` shows the same report).
4. **Contain before diagnosing.** If data or money is at risk, stop the bleeding first:
   freeze deploys, drain or disable the affected route at the gateway, revoke suspected
   credentials. Containment that turns out unnecessary is cheap; diagnosis-first on a breach
   is not.
5. **Preserve evidence.** Do not restart-loop services, delete pods/containers, truncate
   logs or "clean up" rows. Capture logs (Loki), traces (Tempo), audit-log extracts, and
   record deployment versions and config state.
6. **Open comms.** Comms Lead posts the internal declaration (§6.1). If SEV1/2 and
   user-visible, start the school-facing clock.
7. **Pick the next checkpoint.** IC sets a time (15–30 min) by which the team reports
   progress or escalates severity.

## 5. Tenant-isolation-breach playbook (highest severity class)

AuraEDU holds children's education records for many schools in shared databases. Any path by
which one school can read or write another school's data — or a user can act outside their
tenant — is **SEV1 until disproven**, regardless of how small the leak looks. Indicators: a
support report of "wrong school" data; a cross-tenant row in an audit log; an RLS policy
missing or disabled; a `team="security"` alert correlated with unusual access; a failed
isolation gate in CI that matches deployed code.

### 5.1 Immediate containment (minutes, before root cause)

1. If the breach path is a specific route or feature, **disable it at the gateway** (route
   block or feature-flag override from `/superadmin/flags`, reason mandatory). The platform
   fails closed by design — use that.
2. If the breach path is unknown, **drain authenticated traffic** at the gateway rather than
   letting the leak continue while you investigate.
3. Revoke and rotate credentials that could carry cross-tenant scope: service tokens, JWT
   signing key (forces global re-login — acceptable in SEV1), any platform super-admin
   sessions under suspicion.
4. Freeze deploys and database migrations. No code changes except the containment itself.

### 5.2 Evidence preservation (before any cleanup)

- Extract the relevant audit-log rows (cross-tenant explorer at `/superadmin/audit-logs`,
  filtered by `event_type`, `actor_id`, `source_service` and the incident window) and export
  them read-only.
- Capture the exact request/response evidence of the leak (headers, JWT claims, tenant
  codes) and the query or RLS context involved.
- Record database state: RLS enablement/`FORCE` inventory, `app.tenant_id` policies, recent
  migrations, outbox counts. Never delete affected rows or tables.
- Note every identity that may have received foreign-tenant data — this drives notification
  scope.

### 5.3 Assess and eradicate

- Determine the mechanism: missing/incorrect RLS policy, tenant-context leak (missing
  `app.tenant_id`), gateway header handling (`X-Tenant-Code` pinning, `X-Actor-*`
  stripping), token/claim mismatch, or application-level scoping bug.
- Fix with a regression test that fails without the fix, modelled on the EP-50 isolation
  probes (own-resource 200 / cross-resource 404 / mismatch 403, non-enumerating bodies).
- Verify in staging against two real tenants (e.g. the seeded `upshs`/`aboom` shape) before
  production.

### 5.4 Ghana Data Protection Act notification

Personal data of Ghanaian data subjects (students are minors — treat as maximum sensitivity)
is regulated by the **Data Protection Act, 2012 (Act 843)**. Where a breach is likely to
result in risk to data subjects, notify the **Data Protection Commission** without undue
delay — operate on a **72-hour internal deadline from declaration** so the decision and
filing are never last-minute:

1. IC + Comms Lead + legal counsel assess notifiability within 24h: what data, how many
   data subjects, which schools, likelihood of misuse.
2. If notifiable: file with the Data Protection Commission describing the nature of the
   breach, categories and approximate number of data subjects, likely consequences, and
   measures taken. Record the filing reference in the incident log.
3. Notify affected schools (as data controllers' counterparties) with the §6.3 template,
   including what they must tell parents and data subjects.
4. If notification is assessed as **not** required, record the reasoning and the decider —
   an undocumented "we decided not to tell anyone" is itself a governance failure.

## 6. Communication templates

### 6.1 Internal declaration

```
[SEV{n}] Incident declared — {short title}
IC: {name} · Comms: {name} · Started: {timestamp}
Impact: {tenants/services affected, user-visible?}
Containment so far: {actions}
Next update: {time, ≤30 min for SEV1/2}
```

### 6.2 Internal status update

```
[SEV{n}] Update {n} — {timestamp}
Status: {contained / mitigating / investigating}
What changed: {facts only}
User impact now: {…}
Next step + owner: {…} · Next update: {time}
```

### 6.3 School-facing notice (SEV1/2, user-visible)

```
Subject: Service disruption / security notice — {school name}

We are writing about an issue affecting {service} on AuraEDU between {start} and {end/now}.
What happened: {plain language, no speculation}.
Your data: {what was / was not affected — be exact}.
What we have done: {containment and fix}.
What you need to do: {nothing / password reset / …}.
We will update you by {time}. Contact: {support channel}.
```

For a tenant-isolation breach, send only to affected schools, state precisely which records
of theirs were exposed and to whom, and align with the Act 843 filing (§5.4) before sending.

### 6.4 All-clear

```
[SEV{n}] RESOLVED — {short title} — {timestamp}
Duration: {start–end} · Root cause: {one line}
Impact: {final scope} · Follow-ups: {post-incident review date}
```

## 7. Post-incident review (within 5 business days of SEV1/2)

Blameless by rule: we examine systems and decisions, not people.

| Section | Content |
|---|---|
| Summary | Two paragraphs: what happened, what it cost |
| Timeline | From the scribe's log — detection, declaration, containment, resolution |
| Root cause | The mechanism, with `file:line` and the change that introduced it |
| Detection gap | How it was found vs how it should have been found; missing alert? |
| What went well | Keep doing this |
| What went poorly | With evidence, not adjectives |
| Action items | Each with owner + due date; regression tests mandatory for the root cause; tracked like any story (AURA-x.y) |
| Runbook updates | Edits to this runbook, [alerts.md](alerts.md) or [disaster-recovery.md](disaster-recovery.md) |
| Act 843 record | For breaches: filing reference or the documented no-notify decision (§5.4) |

The review is finished when its action items are tickets, not when the document is written.
