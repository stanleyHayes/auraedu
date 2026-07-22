import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, writeFileSync } from "node:fs";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  openLedgerIDs,
  validateIsolationEvidence,
  validateOperationalEvidence,
  validatePerformanceEvidence,
  validateProviderEvidence,
  validateReadiness,
  validateVisualEvidence,
} from "./verify-readiness.mjs";

const ledger = "| AURA-9.1 | Deploy | **In progress** | SRE | proof remains |";
const releaseSHA = "a".repeat(40);

test("extracts every unresolved ledger row regardless of custom status wording", () => {
  assert.deepEqual(
    openLedgerIDs(
      `${ledger}\n| AURA-58.3 | Journeys | **Implemented — browser proof pending** | QA | pending |\n` +
        `| AURA-9.8 | Frontends | **Blocked by provider** | SRE | pending |\n` +
        `| AURA-9.2 | Other | **Done** | SRE | done |`,
    ),
    ["AURA-9.1", "AURA-58.3", "AURA-9.8"],
  );
});

test("pending evidence must match the live ledger", () => {
  const manifest = {
    schema_version: 1,
    items: [
      {
        id: "AURA-9.1",
        status: "pending",
        owner: "SRE",
        requirement: "Retain deployed provider evidence for the production topology.",
        artifacts: [],
      },
    ],
  };
  assert.deepEqual(validateReadiness({ manifest, ledger, repoRoot: "/tmp" }), []);
  assert.match(
    validateReadiness({ manifest, ledger, repoRoot: "/tmp", assertReady: true }).join("\n"),
    /blocked by 1 pending/,
  );
});

test("duplicate live-ledger story IDs fail even when one row says Done", () => {
  const manifest = {
    schema_version: 1,
    items: [
      {
        id: "AURA-9.1",
        status: "pending",
        owner: "SRE",
        requirement: "Retain deployed provider evidence for the production topology.",
        artifacts: [],
      },
    ],
  };
  const duplicate = `${ledger}\n| AURA-9.1 | Old story | **Done** | SRE | done |`;
  assert.match(
    validateReadiness({ manifest, ledger: duplicate, repoRoot: "/tmp" }).join("\n"),
    /duplicate story id in live ledger/,
  );
});

test("every tracked release item must register a strict semantic validator", () => {
  const manifest = {
    schema_version: 1,
    items: [
      {
        id: "AURA-99.9",
        status: "pending",
        owner: "Release",
        requirement: "Retain a substantive but newly introduced external production proof.",
        artifacts: [],
      },
    ],
  };
  const errors = validateReadiness({
    manifest,
    ledger: "| AURA-99.9 | New proof | **In progress** | Release | pending |",
    repoRoot: "/tmp",
  }).join("\n");
  assert.match(errors, /no strict evidence validator is registered/);
});

test("verified artifacts are bounded and checksum verified", async () => {
  const root = await mkdtemp(join(tmpdir(), "auraedu-release-"));
  const directory = join(root, "release/evidence/records/AURA-9.1");
  mkdirSync(directory, { recursive: true });
  const path = join(directory, "proof.json");
  const proof = `${JSON.stringify(
    {
      name: "auraedu-production-render-deployment",
      environment: "production",
      target_url: "https://dashboard.render.com/web/auraedu",
      run_id: "release-2026-07-20-render",
      git_sha: releaseSHA,
      started_at: "2026-07-20T10:00:00Z",
      finished_at: "2026-07-20T10:01:00Z",
      all_passed: true,
      checks: [
        "blueprint-applied",
        "services-healthy",
        "services-ready",
        "identity-login",
        "route-smoke",
        "runtime-config",
        "non-root-runtime",
      ].map((name, index) => ({
        name,
        passed: true,
        observed_at: `2026-07-20T10:00:${String(index).padStart(2, "0")}Z`,
        evidence_fingerprint: String(index + 1).padStart(16, "0"),
      })),
    },
    null,
    2,
  )}\n`;
  writeFileSync(path, proof);
  const digest = createHash("sha256").update(proof).digest("hex");
  const manifest = {
    schema_version: 1,
    release_git_sha: releaseSHA,
    items: [
      {
        id: "AURA-9.1",
        status: "verified",
        owner: "SRE",
        requirement: "Retain deployed provider evidence for the production topology.",
        verified_at: "2026-07-20T10:02:00Z",
        approved_by: "Recovery Lead",
        artifacts: [{ path: "release/evidence/records/AURA-9.1/proof.json", sha256: digest }],
      },
    ],
  };
  assert.deepEqual(
    validateReadiness({ manifest, ledger: "", repoRoot: root, assertReady: true }),
    [],
  );
  manifest.items[0].artifacts[0].sha256 = "0".repeat(64);
  assert.match(
    validateReadiness({ manifest, ledger: "", repoRoot: root }).join("\n"),
    /sha256 mismatch/,
  );
});

test("verified evidence is bound to the exact release candidate commit", async () => {
  const root = await mkdtemp(join(tmpdir(), "auraedu-release-sha-"));
  const directory = join(root, "release/evidence/records/AURA-9.1");
  mkdirSync(directory, { recursive: true });
  const path = join(directory, "proof.json");
  const evidence = {
    name: "auraedu-production-render-deployment",
    environment: "production",
    target_url: "https://dashboard.render.com/web/auraedu",
    run_id: "release-2026-07-20-render",
    git_sha: "b".repeat(40),
    started_at: "2026-07-20T10:00:00Z",
    finished_at: "2026-07-20T10:01:00Z",
    all_passed: true,
    checks: [
      "blueprint-applied",
      "services-healthy",
      "services-ready",
      "identity-login",
      "route-smoke",
      "runtime-config",
      "non-root-runtime",
    ].map((name, index) => ({
      name,
      passed: true,
      observed_at: `2026-07-20T10:00:${String(index).padStart(2, "0")}Z`,
      evidence_fingerprint: String(index + 1).padStart(16, "0"),
    })),
  };
  const contents = `${JSON.stringify(evidence)}\n`;
  writeFileSync(path, contents);
  const digest = createHash("sha256").update(contents).digest("hex");
  const manifest = {
    schema_version: 1,
    release_git_sha: releaseSHA,
    items: [
      {
        id: "AURA-9.1",
        status: "verified",
        owner: "SRE",
        requirement: "Retain deployed provider evidence for the production topology.",
        verified_at: "2026-07-20T10:02:00Z",
        approved_by: "Release Manager",
        artifacts: [{ path: "release/evidence/records/AURA-9.1/proof.json", sha256: digest }],
      },
    ],
  };
  const errors = validateReadiness({
    manifest,
    ledger: "",
    repoRoot: root,
    assertReady: true,
  }).join("\n");
  assert.match(errors, /git_sha must match manifest release_git_sha/);
});

test("verification approval cannot predate the observed proof window", async () => {
  const root = await mkdtemp(join(tmpdir(), "auraedu-release-time-"));
  const directory = join(root, "release/evidence/records/AURA-9.1");
  mkdirSync(directory, { recursive: true });
  const path = join(directory, "proof.json");
  const evidence = {
    name: "auraedu-production-render-deployment",
    environment: "production",
    target_url: "https://dashboard.render.com/web/auraedu",
    run_id: "release-2026-07-20-render",
    git_sha: releaseSHA,
    started_at: "2026-07-20T10:00:00Z",
    finished_at: "2026-07-20T10:01:00Z",
    all_passed: true,
    checks: [
      "blueprint-applied",
      "services-healthy",
      "services-ready",
      "identity-login",
      "route-smoke",
      "runtime-config",
      "non-root-runtime",
    ].map((name, index) => ({
      name,
      passed: true,
      observed_at: `2026-07-20T10:00:${String(index).padStart(2, "0")}Z`,
      evidence_fingerprint: String(index + 1).padStart(16, "0"),
    })),
  };
  const contents = `${JSON.stringify(evidence)}\n`;
  writeFileSync(path, contents);
  const digest = createHash("sha256").update(contents).digest("hex");
  const manifest = {
    schema_version: 1,
    release_git_sha: releaseSHA,
    items: [
      {
        id: "AURA-9.1",
        status: "verified",
        owner: "SRE",
        requirement: "Retain deployed provider evidence for the production topology.",
        verified_at: "2026-07-20T09:59:00Z",
        approved_by: "Release Manager",
        artifacts: [{ path: "release/evidence/records/AURA-9.1/proof.json", sha256: digest }],
      },
    ],
  };
  const errors = validateReadiness({ manifest, ledger: "", repoRoot: root }).join("\n");
  assert.match(errors, /verified_at must be at or after the proof window/);
});

test("operational evidence requires the complete named production proof", () => {
  const evidence = {
    name: "auraedu-production-deployment-hardening",
    environment: "production",
    target_url: "https://dashboard.render.com/web/auraedu",
    run_id: "release-2026-07-20-hardening",
    git_sha: "abcdef1234567890",
    started_at: "2026-07-20T10:00:00Z",
    finished_at: "2026-07-20T10:01:00Z",
    all_passed: true,
    checks: [
      "paid-plans",
      "resource-sizing",
      "autoscaling",
      "health-gated-deploy",
      "zero-downtime-observed",
    ].map((name, index) => ({
      name,
      passed: true,
      observed_at: `2026-07-20T10:00:${String(index).padStart(2, "0")}Z`,
      evidence_fingerprint: String(index + 1).padStart(16, "0"),
    })),
  };
  assert.deepEqual(validateOperationalEvidence("AURA-9.3", evidence), []);
  evidence.checks[2].passed = false;
  evidence.provider_secret = "must-not-be-retained";
  const errors = validateOperationalEvidence("AURA-9.3", evidence).join("\n");
  assert.match(errors, /autoscaling is not passing/);
  assert.match(errors, /unsupported operational field: provider_secret/);
});

test("Twilio evidence proves accepted delivered and persisted state for both channels", () => {
  const names = [
    "sms-provider-accepted",
    "sms-delivered",
    "sms-status-persisted",
    "whatsapp-provider-accepted",
    "whatsapp-delivered",
    "whatsapp-status-persisted",
  ];
  const evidence = {
    name: "auraedu-staging-messaging-providers",
    environment: "staging",
    target_url: "https://staging-api.auraedu.com",
    run_id: "release-2026-07-21-twilio",
    git_sha: "abcdef1234567890",
    started_at: "2026-07-21T10:00:00Z",
    finished_at: "2026-07-21T10:01:00Z",
    all_passed: true,
    checks: names.map((name, index) => ({
      name,
      passed: true,
      observed_at: `2026-07-21T10:00:${String(index).padStart(2, "0")}Z`,
      evidence_fingerprint: String(index + 1).padStart(16, "0"),
    })),
  };
  assert.deepEqual(validateOperationalEvidence("AURA-18.10/18.11", evidence), []);
  evidence.checks[5].name = "status-correlated";
  evidence.phone_number = "+233200000001";
  const errors = validateOperationalEvidence("AURA-18.10/18.11", evidence).join("\n");
  assert.match(errors, /unexpected check: status-correlated/);
  assert.match(errors, /missing required check: whatsapp-status-persisted/);
  assert.match(errors, /unsupported operational field: phone_number/);
});

test("a story with operational and visual obligations cannot verify only one profile", async () => {
  const root = await mkdtemp(join(tmpdir(), "auraedu-release-multi-profile-"));
  const directory = join(root, "release/evidence/records/AURA-59.2");
  mkdirSync(directory, { recursive: true });
  const path = join(directory, "provider.json");
  const evidence = {
    name: "auraedu-staging-governed-content",
    environment: "staging",
    target_url: "https://staging.auraedu.com/admin/content",
    run_id: "release-2026-07-21-content",
    git_sha: releaseSHA,
    started_at: "2026-07-21T10:00:00Z",
    finished_at: "2026-07-21T10:01:00Z",
    all_passed: true,
    checks: [
      "provider-generation",
      "evidence-grounding",
      "compliance-enforced",
      "independent-approval",
      "publish-boundary-denied",
    ].map((name, index) => ({
      name,
      passed: true,
      observed_at: `2026-07-21T10:00:${String(index).padStart(2, "0")}Z`,
      evidence_fingerprint: String(index + 1).padStart(16, "0"),
    })),
  };
  const contents = `${JSON.stringify(evidence)}\n`;
  writeFileSync(path, contents);
  const digest = createHash("sha256").update(contents).digest("hex");
  const manifest = {
    schema_version: 1,
    release_git_sha: releaseSHA,
    items: [
      {
        id: "AURA-59.2",
        status: "verified",
        owner: "AI Platform and Design QA",
        requirement: "Retain provider and visual proof for the governed content workflow.",
        verified_at: "2026-07-21T10:02:00Z",
        approved_by: "Release Manager",
        artifacts: [{ path: "release/evidence/records/AURA-59.2/provider.json", sha256: digest }],
      },
    ],
  };
  const errors = validateReadiness({ manifest, ledger: "", repoRoot: root }).join("\n");
  assert.match(errors, /verified visual evidence requires exactly one JSON result/);
  assert.match(errors, /verified visual evidence requires exactly 8 PNG artifacts/);
});

test("visual evidence requires every named state at each required viewport", () => {
  const states = ["consent", "uncertainty", "locale-en", "locale-fr", "handoff"];
  const captures = states.flatMap((state) =>
    ["desktop", "mobile"].map((viewport) => {
      const screenshot = `release/evidence/records/AURA-57.2/${state}-${viewport}.png`;
      return {
        state,
        viewport,
        width: viewport === "desktop" ? 1440 : 390,
        height: viewport === "desktop" ? 900 : 844,
        route: "/upshs",
        screenshot,
        passed: true,
      };
    }),
  );
  const evidence = {
    name: "auraedu-staging-admissions-assistant-visual",
    environment: "staging",
    base_url: "https://staging.auraedu.com",
    run_id: "release-2026-07-20-assistant",
    git_sha: "abcdef1234567890",
    started_at: "2026-07-20T10:00:00Z",
    finished_at: "2026-07-20T10:01:00Z",
    all_passed: true,
    captures,
  };
  const paths = new Set(captures.map((capture) => capture.screenshot));
  assert.deepEqual(validateVisualEvidence("AURA-57.2", evidence, paths), []);
  paths.delete(captures[0].screenshot);
  captures[1].route = "/upshs?secret=value";
  const errors = validateVisualEvidence("AURA-57.2", evidence, paths).join("\n");
  assert.match(errors, /hashed PNG artifact/);
  assert.match(errors, /query-free application path/);
});

test("performance evidence must prove staging provenance, cardinality and thresholds", () => {
  const tenantMetrics = Object.fromEntries(
    Array.from({ length: 100 }, (_, index) => [
      `perf-${String(index + 1).padStart(3, "0")}`,
      {
        requests: 100,
        failures: 0,
        error_rate: 0,
        p50_ms: 50,
        p95_ms: 100,
        p99_ms: 150,
        requests_per_second: 0.2,
      },
    ]),
  );
  const evidence = {
    name: "auraedu-100-tenant-scale",
    environment: "staging",
    base_url: "https://staging-api.auraedu.com",
    run_id: "release-2026-07-20-scale",
    git_sha: "abcdef1234567890",
    started_at: "2026-07-20T10:00:00Z",
    finished_at: "2026-07-20T10:10:00Z",
    elapsed_ms: 600000,
    required_tenant_count: 100,
    observed_tenant_count: 100,
    requests: 120000,
    failures: 0,
    error_rate: 0,
    p95_ms: 100,
    p99_ms: 150,
    requests_per_second: 200,
    minimum_requests_per_second: 180,
    thresholds: { max_error_rate: 0.01, p95_ms: 750, p99_ms: 1500 },
    by_request: {
      readiness: { requests: 120000, failures: 0, error_rate: 0, p95_ms: 100, p99_ms: 150 },
    },
    by_tenant: tenantMetrics,
  };
  assert.deepEqual(validatePerformanceEvidence("AURA-54.2", evidence), []);
  evidence.requests_per_second = 20;
  evidence.observed_tenant_count = 99;
  const errors = validatePerformanceEvidence("AURA-54.2", evidence).join("\n");
  assert.match(errors, /throughput/);
  assert.match(errors, /tenant counts/);
});

test("isolation evidence must contain the complete two-direction denial matrix without sensitive fields", () => {
  const probes = Array.from({ length: 10 }, (_, index) => `probe-${index + 1}`);
  const checks = probes.flatMap((probe) =>
    ["school-1-to-school-2", "school-2-to-school-1"].flatMap((direction) => [
      { probe, direction, kind: "own-control", status_code: 200, duration_ms: 4, passed: true },
      { probe, direction, kind: "cross-resource", status_code: 404, duration_ms: 3, passed: true },
      {
        probe,
        direction,
        kind: "tenant-header-mismatch",
        status_code: 403,
        duration_ms: 2,
        passed: true,
      },
    ]),
  );
  const evidence = {
    name: "auraedu-staging-two-school-isolation",
    environment: "staging",
    base_url: "https://staging-api.auraedu.com",
    run_id: "release-2026-07-20-isolation",
    git_sha: "abcdef1234567890",
    started_at: "2026-07-20T10:00:00Z",
    finished_at: "2026-07-20T10:01:00Z",
    school_count: 2,
    school_fingerprints: ["1234567890abcdef", "fedcba0987654321"],
    probe_count: 10,
    expected_checks: 60,
    passed_checks: 60,
    failed_checks: 0,
    all_passed: true,
    checks,
  };
  assert.deepEqual(validateIsolationEvidence(evidence), []);
  evidence.checks[1].status_code = 200;
  evidence.token = "must-never-be-retained";
  const errors = validateIsolationEvidence(evidence).join("\n");
  assert.match(errors, /must return 404/);
  assert.match(errors, /unsupported evidence field: token/);
});

test("provider evidence requires accepted, persisted and webhook-delivered state without secrets", () => {
  const evidence = {
    name: "auraedu-staging-email-provider",
    environment: "staging",
    base_url: "https://staging-api.auraedu.com",
    run_id: "release-2026-07-20-email",
    git_sha: "abcdef1234567890",
    started_at: "2026-07-20T10:00:00Z",
    finished_at: "2026-07-20T10:00:03Z",
    tenant_fingerprint: "1234567890abcdef",
    recipient_fingerprint: "fedcba0987654321",
    message_fingerprint: "0011223344556677",
    channel: "email",
    provider: "resend",
    provider_outcome: "accepted",
    persisted_status: "sent",
    sent_at: "2026-07-20T10:00:02Z",
    delivery_status: "delivered",
    delivered_at: "2026-07-20T10:00:03Z",
    all_passed: true,
    steps: [
      { name: "create-message", status_code: 201, duration_ms: 20, passed: true },
      { name: "provider-handoff", status_code: 200, duration_ms: 410, passed: true },
      { name: "persisted-outcome", status_code: 200, duration_ms: 18, passed: true },
      { name: "delivery-feedback", status_code: 200, duration_ms: 640, passed: true },
    ],
  };
  assert.deepEqual(validateProviderEvidence(evidence), []);
  evidence.provider_outcome = "queued";
  evidence.recipient_email = "must-not-be-retained@example.org";
  const errors = validateProviderEvidence(evidence).join("\n");
  assert.match(errors, /Resend acceptance/);
  assert.match(errors, /unsupported provider evidence field: recipient_email/);
});
