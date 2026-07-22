# AuraEDU performance gates

The standard-library load runner applies explicit p95, p99, error-rate and minimum-throughput thresholds and exits non-zero when any threshold fails. It round-robins tenant contexts so aggregate speed cannot hide one school monopolising the service. Percentiles use nearest-rank calculation rather than a floor that can hide the slowest sample in small distributions.

Committed scenarios cover public tenant resolution, authenticated Foundation service reads, Growth catalogue/lead capture, and the exact 100-tenant scaling gate. Results include the sanitized target origin, environment, run ID, Git revision, start/finish times, configured duration/rate/concurrency, thresholds, shutdown drops, tenant cardinality, and aggregate/per-request/per-tenant distributions. Tenant tokens are never written. A result-file write failure fails the run.

Run the two-school critical-path scenario against a local or deployed gateway:

```bash
AURA_PERF_BASE_URL=https://staging-api.example.com \
  go run ./tools/loadtest -config performance/scenarios/critical-paths.json \
  -out performance-result.json
```

The local critical-path scenario does not qualify as release evidence. Every staging run additionally requires an operator-assigned run ID and the exact deployed Git SHA. Placeholder/loopback hosts, plaintext origins, credential-bearing URLs and placeholder authentication tokens fail before traffic is sent:

```bash
AURA_PERF_BASE_URL=https://staging-api.auraedu.com \
AURA_PERF_RUN_ID=release-2026-07-20-authenticated-core \
AURA_PERF_GIT_SHA=<deployed-git-sha> \
AURA_PERF_TENANTS_JSON='<two runtime tenant codes and bearer tokens>' \
  go run ./tools/loadtest -config performance/scenarios/authenticated-core.json \
  -out release/evidence/records/AURA-54.1/release-2026-07-20-authenticated-core.json
```

For the 100-tenant staging proof, provision 100 disposable staging schools through the supported onboarding flow and inject their codes (and tokens for protected scenarios) without committing credentials. The example below generates the required public tenant-code matrix; replace it with the provisioned staging codes before collecting release evidence:

```bash
AURA_PERF_BASE_URL=https://staging-api.auraedu.com \
AURA_PERF_RUN_ID=release-2026-07-20-tenant-scale \
AURA_PERF_GIT_SHA=<deployed-git-sha> \
AURA_PERF_TENANTS_JSON="$(python3 -c 'import json; print(json.dumps([{"code": f"perf-{i:03d}"} for i in range(1, 101)]))')" \
  go run ./tools/loadtest -config performance/scenarios/tenant-scale-100.json \
  -out performance-result.json
```

`AURA_PERF_TENANTS_JSON` must contain exactly 100 unique, actually provisioned staging tenants for the AURA-54.2 release evidence; generated names alone do not prove provisioning. Use a unique run ID in each output filename: the runner refuses to overwrite an existing artifact. Store the resulting JSON below the matching release record and add its SHA-256 to the evidence manifest. Do not commit tenant tokens or production test data.
