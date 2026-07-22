# Chapter 31: Monitoring

## Purpose

Make every production failure visible, diagnosable and owned before it becomes a school-day disruption.

---

## Scope

Metrics, distributed traces, structured logs, dashboards, alerts, liveness, readiness, incident evidence and telemetry retention across APIs, workers, web applications and infrastructure.

---

## Principles

- Observe user outcomes and dependency failures, not merely process uptime.
- Telemetry must preserve tenant isolation and contain no credentials or personal data.
- Alert on actionable symptoms tied to a runbook and owner.
- Keep metric labels bounded. Raw URLs, user IDs, student IDs and arbitrary tenant input are prohibited labels.
- A deployment is not production-ready when its telemetry pipeline has not been exercised in staging.

---

## Business Rules

- The platform API objective is 99.95% monthly availability, less than 1% server errors and p95 latency below 750ms for interactive reads.
- Payment webhook failures are critical because they can leave family balances inconsistent.
- Failed-login spikes are security signals and route to the security owner.
- AI latency and errors are reported separately from deterministic application paths.
- Provider delivery, job failures and tenant usage require domain counters in addition to generic HTTP metrics.

---

## Technical Rules

- Every Go HTTP entrypoint wraps its handler with `observ.HTTPHandler` and initializes `observ.InitTracing` with graceful shutdown.
- `/metrics` uses Prometheus/OpenMetrics. Production deployments set `METRICS_BEARER_TOKEN` unless the endpoint is protected by a private network boundary.
- Route labels come from `net/http` canonical patterns. Raw request paths are never labels.
- OTLP is the transport boundary. Production sampling defaults to 10% and can be changed with `OTEL_TRACES_SAMPLER_ARG`.
- Logs use the shared redacting logger. Trace and request IDs are correlation fields; payload bodies are not.
- Prometheus alert rules, Alertmanager routing and Grafana provisioning are version-controlled under `infrastructure/observability`.

---

## Architecture

Go services expose golden signals to Prometheus and send OTLP spans to the Collector. Active Go workers send bounded job outcomes and notification-provider results as OTLP metrics; the Collector exposes those to Prometheus without tenant or recipient labels. The Collector exports traces to Tempo. Prometheus sends firing and resolved alerts to Alertmanager, which groups and routes them by severity and owning team. Alloy discovers local Docker containers and forwards structured stdout to Loki. Grafana reads Prometheus, Loki and Tempo through provisioned data sources.

Production may replace the local backends with Grafana Cloud, but metric names, alert semantics, sampling rules and redaction requirements remain unchanged.

---

## Best Practices

- Start investigations at the SLO symptom, then pivot from service and route to trace and correlated logs.
- Use p95 for interactive objectives and retain p99 for capacity investigations.
- Test alert expressions against known synthetic failures before enabling paging.
- Record staging dashboard and alert screenshots with release evidence.
- Keep provider and worker metrics close to the code that knows the final outcome.

---

## Examples

- `AuraEDUHighHTTPErrorRate` fires when a service's five-minute 5xx ratio stays above 1% for ten minutes.
- `AuraEDUPaymentWebhookFailures` isolates server failures on Payment webhook routes.
- `AuraEDUNotificationDeliveryFailures` uses the final provider outcome, grouped only by bounded channel and outcome labels.
- A request to `/students/real-student-id` is recorded under `GET /students/{id}`; the identifier never appears in Prometheus.

---

## Anti-patterns

- Treating `/health` returning 200 as evidence that dependencies are ready.
- Labelling metrics with email, tenant name, user ID, record ID or raw URL.
- Logging request or event payloads to make debugging easier.
- Paging on a counter without a duration, rate or operational response.
- Claiming observability complete when dashboards exist but no staging telemetry has reached them.

---

## Checklist

- [ ] Liveness and dependency-aware readiness are distinct.
- [ ] Request rate, errors, duration and saturation are visible per service.
- [ ] Failed login, payment webhook, notification delivery, AI and worker failure signals exist.
- [ ] Metrics and logs contain no PII or credentials.
- [ ] Trace export is configured and sampled for the environment.
- [ ] Dashboard data sources and alert rules validate with their native binaries.
- [ ] Each paging alert links to an owner and runbook.
- [ ] Production Alertmanager receivers use secret-backed paging and communications integrations; local empty receivers are never promoted.
- [ ] Staging evidence proves metric, trace, log and alert flow end to end.

---

## Definition of Done

- The reusable observability CI gate passes.
- Every HTTP and worker entrypoint emits its required signals.
- Prometheus, Grafana, Loki, Tempo, Alloy and Collector configurations validate and boot using pinned images.
- Synthetic staging requests appear in metrics, traces and redacted logs.
- A controlled failure triggers and resolves the expected alert.
- Operations has acknowledged the dashboard, notification route and runbook.

---

## References

- [`platform/observ`](../../../platform/observ/README.md)
- [Observability infrastructure](../../../infrastructure/observability)
- [Performance gates](../../../performance/README.md)
- [Agent execution plan](../../../agent_plan.md)
