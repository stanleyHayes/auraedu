# Chapter 36: Performance

## Purpose

Define measurable performance and capacity rules for AuraEDU so a fast average cannot hide slow-tail requests, errors or one school monopolising shared capacity.

---

## Scope

Gateway and service HTTP latency, background-job throughput, public Growth paths, tenant fairness, database capacity and release-time scaling evidence. Availability, recovery and cost are governed separately but use the same production telemetry.

---

## Principles

- Measure p50, p95 and p99; averages are supporting information only.
- Measure error rate and throughput beside latency.
- Exercise real tenant resolution, feature flags, authentication and RLS paths.
- Distribute load across tenants; never validate scale with one privileged tenant.
- A synthetic benchmark is evidence, not permission to claim an SLA.

---

## Business Rules

- Critical school workflows must remain usable during attendance, assessment and admissions peaks.
- Performance tests use disposable staging data and never send bulk traffic to production without an approved change window.
- A release fails when an adopted threshold is exceeded or required evidence is missing.
- The 99.95 percent availability objective is not a customer SLA until adopted through an SLA ADR and backed by production monitoring.

---

## Technical Rules

- Default interactive-read release thresholds are error rate at most 1 percent, p95 at most 750 ms and p99 at most 1,500 ms.
- Go service ingress caps standard request bodies at 1 MiB and multipart requests at 40 MiB before handler decoding. Domain handlers keep stricter limits where their contract permits less data.
- Go internal service clients cap decoded JSON responses at 1 MiB so dependency failures cannot turn into unbounded caller allocation.
- Python AI APIs cap declared and streamed request bodies at 1 MiB before Pydantic/FastAPI decoding and record rejected requests in the same low-cardinality HTTP metrics.
- Endpoint-specific stricter thresholds may be added; a scenario may not weaken these defaults without an ADR.
- Client timeout is five seconds for the standard scenarios.
- Scenarios declare an arrival rate so public abuse controls and expected peak traffic are tested deliberately rather than overwhelmed accidentally by an unbounded local loop.
- The load generator omits requests cancelled only because the configured scenario duration ended; client timeouts and unexpected statuses remain failures.
- Protected scenarios inject tenant tokens at runtime. Tokens and performance result files containing environment details are not committed.
- AURA-54.2 requires exactly 100 unique disposable tenants. The runner rejects an undersized tenant list.
- Every measured request sends both canonical tenant headers and round-robins all supplied tenants.

---

## Architecture

The standard-library runner in `tools/loadtest` reads versioned JSON scenarios, sends bounded concurrent traffic, drains response bodies for connection reuse, records latency/status samples and exits non-zero when thresholds fail. Scenario validation runs in CI. Traffic execution runs against local integration or staging environments and emits a JSON release artifact.

---

## Best Practices

- Warm the environment before recording a baseline, then run at least three comparable trials.
- Separate public reads, authenticated reads, writes and asynchronous completion scenarios.
- Correlate regressions with service, database, Redis, NATS and provider telemetry.
- Compare tenant-level latency distributions as well as the aggregate result.
- Record configuration, commit SHA, environment, start time and result artifact with the release.

---

## Examples

Validate the committed two-school critical-path scenario:

```bash
go run ./tools/loadtest \
  -config performance/scenarios/critical-paths.json \
  -validate-only
```

Execute it against staging and write evidence:

```bash
AURA_PERF_BASE_URL=https://staging-api.example.com \
  go run ./tools/loadtest \
  -config performance/scenarios/critical-paths.json \
  -out performance-result.json
```

---

## Anti-patterns

- Reporting average latency without tail latency and error rate.
- Calling a two-tenant run a 100-tenant capacity test.
- Testing service health endpoints only and claiming business-path capacity.
- using platform-super-admin credentials for normal school traffic.
- Silently accepting `401`, `403`, `429` or `5xx` as successful load samples.
- Allowing an unbounded request body to consume handler memory or connection time.

---

## Checklist

- Does the scenario cover the real gateway and tenant path?
- Are expected statuses explicit for every request?
- Are p95, p99, error-rate and timeout thresholds defined?
- Are all required tenants unique and represented?
- Are result artifacts tied to an environment and commit?
- Were regressions investigated rather than averaged away?

---

## Definition of Done

- Runner unit tests and scenario validation pass in CI.
- Critical endpoint classes have committed scenarios and named owners.
- A staging run meets its thresholds and its JSON result is retained with the release.
- The 100-tenant scenario receives exactly 100 provisioned tenant contexts and passes without cross-tenant errors or material tenant-level starvation.
- Relevant dashboards show matching latency, error and saturation signals during the run.

---

## References

- [Performance runner](../../../tools/loadtest)
- [Versioned scenarios](../../../performance/scenarios)
- [Non-functional requirements](../01-foundations/05-non-functional-requirements.md)
- [Agent execution plan](../../../agent_plan.md)
