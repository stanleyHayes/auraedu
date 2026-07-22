# AuraEDU observability

`platform/observ` is the mandatory server observability boundary for AuraEDU Go services.

`HTTPHandler(service, next)` exposes Prometheus/OpenMetrics on `/metrics`, creates server spans and records request count, in-flight requests and duration. Metrics use the canonical `net/http` route pattern rather than the raw URL, preventing record IDs or attacker-controlled paths from becoming high-cardinality labels. Set `METRICS_BEARER_TOKEN` to protect the endpoint outside a private network.

`InitTracing(service, version)` initializes the process telemetry providers. Traces export through OTLP when `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is configured; metrics export when the common endpoint or `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` is configured. Tracing defaults to 10% sampling in production, honours `OTEL_TRACES_SAMPLER_ARG`, preserves parent sampling decisions and flushes both providers during graceful shutdown. With no exporter configured it installs a non-recording trace provider and performs no metric network activity.

`NewWorkerMetrics(service, allowedJobs...)` records job counts and durations with a fixed label allowlist. Undeclared runtime values collapse to `unknown`; callers must never pass tenant, user, event or record identifiers as declared jobs.

`DefaultLogger` installs structured JSON logging with field and message redaction. Logs must never contain tenant payloads, tokens, credentials, email addresses or phone numbers.

Local telemetry runs through `deploy/docker-compose.infra.yml`:

- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Grafana: `http://localhost:3000`
- Loki: `http://localhost:3100`
- Tempo: `http://localhost:3200`
- Alloy diagnostics: `http://localhost:12345`

Run the complete structural and native configuration gate with:

```bash
AURA_OBSERVABILITY_CONTAINER_VALIDATE=1 ./tools/ci/run-observability-tests.sh
```
