#!/usr/bin/env node

import { Buffer } from "node:buffer";
import console from "node:console";
import { createHash } from "node:crypto";
import { readFileSync, statSync } from "node:fs";
import { dirname, isAbsolute, relative, resolve } from "node:path";
import process from "node:process";
import { fileURLToPath, URL } from "node:url";

const idPattern = /^AURA-[0-9]+(?:\.[0-9]+(?:\/[0-9]+\.[0-9]+)?)?$/;
const shaPattern = /^[a-f0-9]{64}$/;

export function ledgerStatusRows(markdown) {
  const rows = [];
  for (const line of markdown.split(/\r?\n/)) {
    if (!line.startsWith("|")) continue;
    const cells = line
      .split("|")
      .slice(1, -1)
      .map((cell) => cell.trim());
    if (cells.length < 3 || !cells[0].startsWith("AURA-")) continue;
    const status = cells[2].match(/^\*\*(.+)\*\*$/)?.[1]?.trim();
    if (status) rows.push({ id: cells[0], status });
  }
  return rows;
}

export function openLedgerIDs(markdown) {
  return ledgerStatusRows(markdown)
    .filter(({ status }) => status !== "Done")
    .map(({ id }) => id);
}

function sha256(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

function safeArtifactPath(repoRoot, input) {
  if (typeof input !== "string" || input.length === 0 || isAbsolute(input)) {
    throw new Error("artifact path must be a non-empty repository-relative path");
  }
  const absolute = resolve(repoRoot, input);
  const rel = relative(repoRoot, absolute);
  if (rel.startsWith("..") || isAbsolute(rel) || !rel.startsWith("release/evidence/records/")) {
    throw new Error(`artifact path must stay below release/evidence/records: ${input}`);
  }
  return absolute;
}

const performanceExpectations = {
  "AURA-54.1": { name: "auraedu-authenticated-core", tenants: 2 },
  "AURA-54.2": { name: "auraedu-100-tenant-scale", tenants: 100 },
};

const operationalExpectations = {
  "AURA-8.1": {
    name: "auraedu-staging-observability",
    environment: "staging",
    checks: ["metrics", "traces", "logs", "dashboard", "alert-fired", "paging-delivered"],
  },
  "AURA-48.7": {
    name: "auraedu-mobile-store-release",
    environment: "production",
    checks: [
      "eas-project-linked",
      "credentials-validated",
      "ios-signed-build",
      "android-signed-build",
      "app-store-connect-submitted",
      "google-play-submitted",
    ],
  },
  "AURA-48.8": {
    name: "auraedu-mobile-ota-promotion",
    environment: "production",
    checks: [
      "preview-published",
      "preview-installed",
      "production-promoted",
      "production-installed",
    ],
  },
  "AURA-47.3": {
    name: "auraedu-staging-first-admin-onboarding",
    environment: "staging",
    checks: [
      "signup-submitted",
      "tenant-approved",
      "invite-provider-accepted",
      "invite-opened",
      "admin-activated",
    ],
  },
  "AURA-59.2": {
    name: "auraedu-staging-governed-content",
    environment: "staging",
    checks: [
      "provider-generation",
      "evidence-grounding",
      "compliance-enforced",
      "independent-approval",
      "publish-boundary-denied",
    ],
  },
  "AURA-18.10/18.11": {
    name: "auraedu-staging-messaging-providers",
    environment: "staging",
    checks: [
      "sms-provider-accepted",
      "sms-delivered",
      "sms-status-persisted",
      "whatsapp-provider-accepted",
      "whatsapp-delivered",
      "whatsapp-status-persisted",
    ],
  },
  "AURA-9.3": {
    name: "auraedu-production-deployment-hardening",
    environment: "production",
    checks: [
      "paid-plans",
      "resource-sizing",
      "autoscaling",
      "health-gated-deploy",
      "zero-downtime-observed",
    ],
  },
  "AURA-9.4": {
    name: "auraedu-production-disaster-recovery",
    environment: "production",
    checks: [
      "pitr-enabled",
      "postgres-backup-run",
      "nats-backup-run",
      "object-lock-inspected",
      "heartbeat-delivered",
      "page-delivered",
      "restore-completed",
      "cutover-completed",
      "rollback-tested",
    ],
  },
  "AURA-9.5": {
    name: "auraedu-production-custom-domain",
    environment: "production",
    checks: [
      "domain-provisioned",
      "dns-verified",
      "certificate-issued",
      "activated",
      "https-routed",
      "cors-exact-origin",
      "deactivated",
    ],
  },
  "AURA-9.1": {
    name: "auraedu-production-render-deployment",
    environment: "production",
    checks: [
      "blueprint-applied",
      "services-healthy",
      "services-ready",
      "identity-login",
      "route-smoke",
      "runtime-config",
      "non-root-runtime",
    ],
  },
  "AURA-9.8": {
    name: "auraedu-production-vercel-frontends",
    environment: "production",
    checks: [
      "web-project-linked",
      "marketing-project-linked",
      "environment-configured",
      "web-production-deployed",
      "marketing-production-deployed",
      "gateway-cors-observed",
    ],
  },
};

const visualExpectations = {
  "AURA-57.2": {
    name: "auraedu-staging-admissions-assistant-visual",
    states: ["consent", "uncertainty", "locale-en", "locale-fr", "handoff"],
    viewports: ["desktop", "mobile"],
  },
  "AURA-58.1": {
    name: "auraedu-staging-admissions-conversion-visual",
    states: [
      "applicant-draft",
      "applicant-checklist",
      "staff-pipeline",
      "staff-decision",
      "offer-acceptance",
    ],
    viewports: ["desktop", "mobile"],
  },
  "AURA-58.3": {
    name: "auraedu-staging-communication-journeys-visual",
    states: ["builder", "active-monitoring", "provider-outcomes", "unsubscribe-success"],
    viewports: ["desktop", "mobile"],
  },
  "AURA-59.2": {
    name: "auraedu-staging-governed-content-visual",
    states: ["brand-policy", "evidence-brief", "compliance-review", "approval-history"],
    viewports: ["desktop", "mobile"],
  },
  "AURA-21.9": {
    name: "auraedu-staging-teacher-analytics-visual",
    states: ["populated", "empty", "dependency-failure"],
    viewports: ["desktop"],
  },
};

const supportedEvidenceIDs = new Set([
  ...Object.keys(performanceExpectations),
  ...Object.keys(operationalExpectations),
  ...Object.keys(visualExpectations),
  "AURA-50.2",
  "AURA-18.9",
]);

function finiteNumber(value) {
  return typeof value === "number" && Number.isFinite(value);
}

export function validatePerformanceEvidence(itemID, evidence) {
  const expected = performanceExpectations[itemID];
  if (!expected) return [];
  const errors = [];
  if (evidence?.name !== expected.name) errors.push(`scenario must be ${expected.name}`);
  if (evidence?.environment !== "staging") errors.push("environment must be staging");
  try {
    const target = new URL(evidence?.base_url);
    if (
      target.protocol !== "https:" ||
      target.username ||
      target.password ||
      target.search ||
      target.hash
    ) {
      errors.push("base_url must be a credential-free HTTPS origin");
    }
    if (
      target.hostname === "localhost" ||
      target.hostname === "127.0.0.1" ||
      target.hostname.endsWith(".example")
    ) {
      errors.push("base_url cannot be loopback or a placeholder");
    }
  } catch {
    errors.push("base_url must be an absolute URL");
  }
  if (typeof evidence?.run_id !== "string" || evidence.run_id.length < 8)
    errors.push("run_id is required");
  if (!/^[0-9a-f]{7,64}$/.test(evidence?.git_sha ?? "")) errors.push("git_sha is invalid");
  if (
    Number.isNaN(Date.parse(evidence?.started_at)) ||
    Number.isNaN(Date.parse(evidence?.finished_at))
  ) {
    errors.push("started_at and finished_at must be RFC 3339 timestamps");
  }
  if (!finiteNumber(evidence?.elapsed_ms) || evidence.elapsed_ms <= 0)
    errors.push("elapsed_ms must be positive");
  if (
    evidence?.required_tenant_count !== expected.tenants ||
    evidence?.observed_tenant_count !== expected.tenants
  ) {
    errors.push(`required and observed tenant counts must both equal ${expected.tenants}`);
  }
  if (Object.keys(evidence?.by_tenant ?? {}).length !== expected.tenants)
    errors.push(`by_tenant must contain exactly ${expected.tenants} tenants`);
  if (!finiteNumber(evidence?.requests) || evidence.requests <= 0)
    errors.push("requests must be positive");
  if (
    !finiteNumber(evidence?.minimum_requests_per_second) ||
    evidence.minimum_requests_per_second <= 0
  )
    errors.push("minimum throughput must be positive");
  if (
    !finiteNumber(evidence?.requests_per_second) ||
    evidence.requests_per_second < evidence.minimum_requests_per_second
  )
    errors.push("observed throughput is below the configured minimum");
  const thresholds = evidence?.thresholds ?? {};
  if (
    !finiteNumber(thresholds.max_error_rate) ||
    !finiteNumber(thresholds.p95_ms) ||
    !finiteNumber(thresholds.p99_ms)
  ) {
    errors.push("error-rate, p95 and p99 thresholds are required");
  }
  const distributions = [
    ["aggregate", evidence],
    ...Object.entries(evidence?.by_request ?? {}).map(([name, value]) => [
      `request ${name}`,
      value,
    ]),
    ...Object.entries(evidence?.by_tenant ?? {}).map(([name, value]) => [`tenant ${name}`, value]),
  ];
  if (distributions.length < 3) errors.push("request and tenant distributions are required");
  for (const [scope, value] of distributions) {
    if (!finiteNumber(value?.error_rate) || value.error_rate > thresholds.max_error_rate)
      errors.push(`${scope} exceeds max_error_rate`);
    if (!finiteNumber(value?.p95_ms) || value.p95_ms > thresholds.p95_ms)
      errors.push(`${scope} exceeds p95_ms`);
    if (!finiteNumber(value?.p99_ms) || value.p99_ms > thresholds.p99_ms)
      errors.push(`${scope} exceeds p99_ms`);
  }
  return errors;
}

const isolationEvidenceKeys = new Set([
  "name",
  "environment",
  "base_url",
  "run_id",
  "git_sha",
  "started_at",
  "finished_at",
  "school_count",
  "school_fingerprints",
  "probe_count",
  "expected_checks",
  "passed_checks",
  "failed_checks",
  "all_passed",
  "checks",
]);
const isolationCheckKeys = new Set([
  "probe",
  "direction",
  "kind",
  "status_code",
  "duration_ms",
  "passed",
]);

function validateStagingOrigin(value, errors) {
  try {
    const target = new URL(value);
    if (
      target.protocol !== "https:" ||
      target.username ||
      target.password ||
      target.search ||
      target.hash ||
      (target.pathname !== "/" && target.pathname !== "")
    ) {
      errors.push("base_url must be a credential-free HTTPS origin");
    }
    if (
      ["localhost", "127.0.0.1", "::1"].includes(target.hostname) ||
      target.hostname.endsWith(".example")
    ) {
      errors.push("base_url cannot be loopback or a placeholder");
    }
  } catch {
    errors.push("base_url must be an absolute URL");
  }
}

export function validateIsolationEvidence(evidence) {
  const errors = [];
  if (evidence?.name !== "auraedu-staging-two-school-isolation")
    errors.push("scenario name is invalid");
  if (evidence?.environment !== "staging") errors.push("environment must be staging");
  validateStagingOrigin(evidence?.base_url, errors);
  if (
    typeof evidence?.run_id !== "string" ||
    !/^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$/.test(evidence.run_id)
  )
    errors.push("run_id is invalid");
  if (!/^[0-9a-f]{7,64}$/.test(evidence?.git_sha ?? "")) errors.push("git_sha is invalid");
  const started = Date.parse(evidence?.started_at);
  const finished = Date.parse(evidence?.finished_at);
  if (Number.isNaN(started) || Number.isNaN(finished) || finished < started)
    errors.push("started_at and finished_at must be ordered RFC 3339 timestamps");
  if (evidence?.school_count !== 2) errors.push("school_count must equal two");
  if (
    !Array.isArray(evidence?.school_fingerprints) ||
    evidence.school_fingerprints.length !== 2 ||
    new Set(evidence.school_fingerprints).size !== 2 ||
    evidence.school_fingerprints.some((value) => !/^[0-9a-f]{16}$/.test(value))
  ) {
    errors.push("two distinct sanitized school fingerprints are required");
  }
  if (!Number.isInteger(evidence?.probe_count) || evidence.probe_count < 10)
    errors.push("at least ten probes are required");
  const expectedChecks = Number.isInteger(evidence?.probe_count) ? evidence.probe_count * 6 : -1;
  if (evidence?.expected_checks !== expectedChecks)
    errors.push("expected_checks must equal six checks per probe");
  if (
    evidence?.passed_checks !== expectedChecks ||
    evidence?.failed_checks !== 0 ||
    evidence?.all_passed !== true
  ) {
    errors.push("every isolation check must pass");
  }
  if (!Array.isArray(evidence?.checks) || evidence.checks.length !== expectedChecks)
    errors.push("checks must contain the complete matrix");

  for (const key of Object.keys(evidence ?? {})) {
    if (!isolationEvidenceKeys.has(key)) errors.push(`unsupported evidence field: ${key}`);
  }
  const combinations = new Set();
  const probes = new Set();
  const expectedStatuses = {
    "own-control": 200,
    "cross-resource": 404,
    "tenant-header-mismatch": 403,
  };
  for (const check of Array.isArray(evidence?.checks) ? evidence.checks : []) {
    for (const key of Object.keys(check ?? {})) {
      if (!isolationCheckKeys.has(key)) errors.push(`unsupported check field: ${key}`);
    }
    if (typeof check?.probe !== "string" || check.probe.length === 0)
      errors.push("every check requires a probe name");
    else probes.add(check.probe);
    if (!["school-1-to-school-2", "school-2-to-school-1"].includes(check?.direction))
      errors.push(`invalid direction for ${check?.probe ?? "check"}`);
    if (!(check?.kind in expectedStatuses))
      errors.push(`invalid check kind for ${check?.probe ?? "check"}`);
    else if (check.status_code !== expectedStatuses[check.kind])
      errors.push(`${check.probe}: ${check.kind} must return ${expectedStatuses[check.kind]}`);
    if (check?.passed !== true) errors.push(`${check?.probe ?? "check"}: check is not passing`);
    if (!Number.isInteger(check?.duration_ms) || check.duration_ms < 0)
      errors.push(`${check?.probe ?? "check"}: duration_ms is invalid`);
    const combination = `${check?.probe}|${check?.direction}|${check?.kind}`;
    if (combinations.has(combination)) errors.push(`duplicate isolation check: ${combination}`);
    combinations.add(combination);
  }
  if (Number.isInteger(evidence?.probe_count) && probes.size !== evidence.probe_count)
    errors.push("probe_count does not match the distinct checks");
  return errors;
}

const providerEvidenceKeys = new Set([
  "name",
  "environment",
  "base_url",
  "run_id",
  "git_sha",
  "started_at",
  "finished_at",
  "tenant_fingerprint",
  "recipient_fingerprint",
  "message_fingerprint",
  "channel",
  "provider",
  "provider_outcome",
  "persisted_status",
  "sent_at",
  "delivery_status",
  "delivered_at",
  "all_passed",
  "steps",
]);
const providerStepKeys = new Set(["name", "status_code", "duration_ms", "passed"]);

export function validateProviderEvidence(evidence) {
  const errors = [];
  if (evidence?.name !== "auraedu-staging-email-provider") errors.push("scenario name is invalid");
  if (evidence?.environment !== "staging") errors.push("environment must be staging");
  validateStagingOrigin(evidence?.base_url, errors);
  if (
    typeof evidence?.run_id !== "string" ||
    !/^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$/.test(evidence.run_id)
  )
    errors.push("run_id is invalid");
  if (!/^[0-9a-f]{7,64}$/.test(evidence?.git_sha ?? "")) errors.push("git_sha is invalid");
  const started = Date.parse(evidence?.started_at);
  const finished = Date.parse(evidence?.finished_at);
  const sent = Date.parse(evidence?.sent_at);
  const delivered = Date.parse(evidence?.delivered_at);
  if (Number.isNaN(started) || Number.isNaN(finished) || finished < started)
    errors.push("started_at and finished_at must be ordered RFC 3339 timestamps");
  if (Number.isNaN(sent) || sent < started || sent > finished)
    errors.push("sent_at must fall inside the proof window");
  if (Number.isNaN(delivered) || delivered < sent || delivered > finished)
    errors.push("delivered_at must follow sent_at inside the proof window");
  const fingerprints = [
    evidence?.tenant_fingerprint,
    evidence?.recipient_fingerprint,
    evidence?.message_fingerprint,
  ];
  if (
    fingerprints.some((value) => !/^[0-9a-f]{16}$/.test(value ?? "")) ||
    new Set(fingerprints).size !== 3
  ) {
    errors.push("three distinct sanitized fingerprints are required");
  }
  if (
    evidence?.channel !== "email" ||
    evidence?.provider !== "resend" ||
    evidence?.provider_outcome !== "accepted" ||
    evidence?.persisted_status !== "sent" ||
    evidence?.delivery_status !== "delivered" ||
    evidence?.all_passed !== true
  ) {
    errors.push(
      "Resend acceptance, persisted sent status and delivered webhook feedback are required",
    );
  }
  for (const key of Object.keys(evidence ?? {})) {
    if (!providerEvidenceKeys.has(key)) errors.push(`unsupported provider evidence field: ${key}`);
  }
  const expectedSteps = new Map([
    ["create-message", 201],
    ["provider-handoff", 200],
    ["persisted-outcome", 200],
    ["delivery-feedback", 200],
  ]);
  if (!Array.isArray(evidence?.steps) || evidence.steps.length !== expectedSteps.size)
    errors.push("provider proof must contain exactly four steps");
  const seen = new Set();
  for (const step of Array.isArray(evidence?.steps) ? evidence.steps : []) {
    for (const key of Object.keys(step ?? {})) {
      if (!providerStepKeys.has(key)) errors.push(`unsupported provider step field: ${key}`);
    }
    if (!expectedSteps.has(step?.name))
      errors.push(`invalid provider step: ${step?.name ?? "<missing>"}`);
    else if (step.status_code !== expectedSteps.get(step.name))
      errors.push(`${step.name} returned the wrong status`);
    if (step?.passed !== true) errors.push(`${step?.name ?? "provider step"} is not passing`);
    if (!Number.isInteger(step?.duration_ms) || step.duration_ms < 0)
      errors.push(`${step?.name ?? "provider step"} duration_ms is invalid`);
    if (seen.has(step?.name)) errors.push(`duplicate provider step: ${step?.name}`);
    seen.add(step?.name);
  }
  if (seen.size !== expectedSteps.size) errors.push("provider proof is missing a required step");
  return errors;
}

const operationalEvidenceKeys = new Set([
  "name",
  "environment",
  "target_url",
  "run_id",
  "git_sha",
  "started_at",
  "finished_at",
  "all_passed",
  "checks",
]);
const operationalCheckKeys = new Set(["name", "passed", "observed_at", "evidence_fingerprint"]);

function validateProofWindow(evidence, errors) {
  if (
    typeof evidence?.run_id !== "string" ||
    !/^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$/.test(evidence.run_id)
  ) {
    errors.push("run_id is invalid");
  }
  if (!/^[0-9a-f]{7,64}$/.test(evidence?.git_sha ?? "")) errors.push("git_sha is invalid");
  const started = Date.parse(evidence?.started_at);
  const finished = Date.parse(evidence?.finished_at);
  if (Number.isNaN(started) || Number.isNaN(finished) || finished < started) {
    errors.push("started_at and finished_at must be ordered RFC 3339 timestamps");
  }
  return { started, finished };
}

function validateHTTPSURL(value, field, errors) {
  try {
    const target = new URL(value);
    if (
      target.protocol !== "https:" ||
      target.username ||
      target.password ||
      target.search ||
      target.hash
    ) {
      errors.push(`${field} must be a credential-free HTTPS URL without query or fragment`);
    }
    if (
      ["localhost", "127.0.0.1", "::1"].includes(target.hostname) ||
      target.hostname.endsWith(".example")
    ) {
      errors.push(`${field} cannot be loopback or a placeholder`);
    }
  } catch {
    errors.push(`${field} must be an absolute URL`);
  }
}

export function validateOperationalEvidence(itemID, evidence) {
  const expected = operationalExpectations[itemID];
  if (!expected) return [`no operational evidence profile exists for ${itemID}`];
  const errors = [];
  if (evidence?.name !== expected.name) errors.push(`scenario must be ${expected.name}`);
  if (evidence?.environment !== expected.environment)
    errors.push(`environment must be ${expected.environment}`);
  validateHTTPSURL(evidence?.target_url, "target_url", errors);
  const { started, finished } = validateProofWindow(evidence, errors);
  if (evidence?.all_passed !== true) errors.push("all_passed must be true");
  for (const key of Object.keys(evidence ?? {})) {
    if (!operationalEvidenceKeys.has(key)) errors.push(`unsupported operational field: ${key}`);
  }
  const expectedChecks = new Set(expected.checks);
  if (!Array.isArray(evidence?.checks) || evidence.checks.length !== expectedChecks.size) {
    errors.push(`checks must contain exactly ${expectedChecks.size} required results`);
  }
  const seen = new Set();
  for (const check of Array.isArray(evidence?.checks) ? evidence.checks : []) {
    for (const key of Object.keys(check ?? {})) {
      if (!operationalCheckKeys.has(key))
        errors.push(`unsupported operational check field: ${key}`);
    }
    if (!expectedChecks.has(check?.name))
      errors.push(`unexpected check: ${check?.name ?? "<missing>"}`);
    if (seen.has(check?.name)) errors.push(`duplicate check: ${check?.name}`);
    seen.add(check?.name);
    if (check?.passed !== true) errors.push(`${check?.name ?? "check"} is not passing`);
    const observed = Date.parse(check?.observed_at);
    if (Number.isNaN(observed) || observed < started || observed > finished)
      errors.push(`${check?.name ?? "check"} observed_at must fall inside the proof window`);
    if (!/^[0-9a-f]{16}$/.test(check?.evidence_fingerprint ?? ""))
      errors.push(`${check?.name ?? "check"} requires a sanitized evidence fingerprint`);
  }
  for (const name of expectedChecks) {
    if (!seen.has(name)) errors.push(`missing required check: ${name}`);
  }
  return errors;
}

const visualEvidenceKeys = new Set([
  "name",
  "environment",
  "base_url",
  "run_id",
  "git_sha",
  "started_at",
  "finished_at",
  "all_passed",
  "captures",
]);
const visualCaptureKeys = new Set([
  "state",
  "viewport",
  "width",
  "height",
  "route",
  "screenshot",
  "passed",
]);

export function validateVisualEvidence(itemID, evidence, artifactPaths = new Set()) {
  const expected = visualExpectations[itemID];
  if (!expected) return [`no visual evidence profile exists for ${itemID}`];
  const errors = [];
  if (evidence?.name !== expected.name) errors.push(`scenario must be ${expected.name}`);
  if (evidence?.environment !== "staging") errors.push("environment must be staging");
  validateStagingOrigin(evidence?.base_url, errors);
  validateProofWindow(evidence, errors);
  if (evidence?.all_passed !== true) errors.push("all_passed must be true");
  for (const key of Object.keys(evidence ?? {})) {
    if (!visualEvidenceKeys.has(key)) errors.push(`unsupported visual field: ${key}`);
  }
  const required = new Set(
    expected.states.flatMap((state) =>
      expected.viewports.map((viewport) => `${state}|${viewport}`),
    ),
  );
  if (!Array.isArray(evidence?.captures) || evidence.captures.length !== required.size) {
    errors.push(`captures must contain exactly ${required.size} required screenshots`);
  }
  const seen = new Set();
  const screenshots = new Set();
  for (const capture of Array.isArray(evidence?.captures) ? evidence.captures : []) {
    for (const key of Object.keys(capture ?? {})) {
      if (!visualCaptureKeys.has(key)) errors.push(`unsupported visual capture field: ${key}`);
    }
    const combination = `${capture?.state}|${capture?.viewport}`;
    if (!required.has(combination)) errors.push(`unexpected visual capture: ${combination}`);
    if (seen.has(combination)) errors.push(`duplicate visual capture: ${combination}`);
    seen.add(combination);
    const desktop = capture?.viewport === "desktop";
    if (
      !Number.isInteger(capture?.width) ||
      !Number.isInteger(capture?.height) ||
      capture.width < (desktop ? 1280 : 320) ||
      capture.height < (desktop ? 720 : 568) ||
      (desktop ? capture.width <= capture.height : capture.width >= capture.height)
    ) {
      errors.push(`${combination} has an invalid viewport`);
    }
    if (
      typeof capture?.route !== "string" ||
      !capture.route.startsWith("/") ||
      capture.route.includes("?") ||
      capture.route.includes("#")
    ) {
      errors.push(`${combination} route must be a query-free application path`);
    }
    if (
      typeof capture?.screenshot !== "string" ||
      !capture.screenshot.endsWith(".png") ||
      !artifactPaths.has(capture.screenshot)
    ) {
      errors.push(`${combination} screenshot must reference a hashed PNG artifact`);
    } else if (screenshots.has(capture.screenshot)) {
      errors.push(`${combination} reuses another capture's screenshot`);
    } else {
      screenshots.add(capture.screenshot);
    }
    if (capture?.passed !== true) errors.push(`${combination} is not passing`);
  }
  for (const combination of required) {
    if (!seen.has(combination)) errors.push(`missing required visual capture: ${combination}`);
  }
  return errors;
}

function pngDimensions(path) {
  const value = readFileSync(path);
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  const ending = Buffer.from([73, 69, 78, 68, 174, 66, 96, 130]);
  if (
    value.length < 45 ||
    !value.subarray(0, 8).equals(signature) ||
    value.subarray(12, 16).toString("ascii") !== "IHDR" ||
    !value.subarray(-8).equals(ending)
  )
    throw new Error("screenshot is not a PNG");
  return { width: value.readUInt32BE(16), height: value.readUInt32BE(20) };
}

function expectedEvidenceNames(itemID) {
  const names = new Set();
  if (performanceExpectations[itemID]) names.add(performanceExpectations[itemID].name);
  if (operationalExpectations[itemID]) names.add(operationalExpectations[itemID].name);
  if (visualExpectations[itemID]) names.add(visualExpectations[itemID].name);
  if (itemID === "AURA-50.2") names.add("auraedu-staging-two-school-isolation");
  if (itemID === "AURA-18.9") names.add("auraedu-staging-email-provider");
  return names;
}

export function validateReadiness({ manifest, ledger, repoRoot, assertReady = false }) {
  const errors = [];
  if (manifest?.schema_version !== 1 || !Array.isArray(manifest?.items)) {
    return ["manifest must use schema_version 1 and contain an items array"];
  }

  const verifiedItems = manifest.items.filter((item) => item?.status === "verified");
  const releaseGitSHA = manifest.release_git_sha;
  const releaseGitSHAValid = /^[0-9a-f]{40}$/.test(releaseGitSHA ?? "");
  if ((verifiedItems.length > 0 || assertReady) && !releaseGitSHAValid) {
    errors.push("release_git_sha must be the exact 40-character release candidate commit");
  } else if (releaseGitSHA != null && !releaseGitSHAValid) {
    errors.push("release_git_sha must be null or an exact 40-character commit");
  }

  const seen = new Set();
  const pending = new Set();
  for (const item of manifest.items) {
    if (!idPattern.test(item.id ?? "")) errors.push(`invalid item id: ${item.id ?? "<missing>"}`);
    if (seen.has(item.id)) errors.push(`duplicate item id: ${item.id}`);
    seen.add(item.id);
    if (!["pending", "verified"].includes(item.status))
      errors.push(`${item.id}: status must be pending or verified`);
    if (typeof item.owner !== "string" || item.owner.trim().length < 2)
      errors.push(`${item.id}: owner is required`);
    if (typeof item.requirement !== "string" || item.requirement.trim().length < 20)
      errors.push(`${item.id}: substantive requirement is required`);
    if (!Array.isArray(item.artifacts)) errors.push(`${item.id}: artifacts must be an array`);
    if (idPattern.test(item.id ?? "") && !supportedEvidenceIDs.has(item.id))
      errors.push(`${item.id}: no strict evidence validator is registered`);

    if (item.status === "pending") pending.add(item.id);
    if (item.status === "verified") {
      if (!item.verified_at || Number.isNaN(Date.parse(item.verified_at)))
        errors.push(`${item.id}: verified_at must be RFC 3339 compatible`);
      if (typeof item.approved_by !== "string" || item.approved_by.trim().length < 2)
        errors.push(`${item.id}: approved_by is required`);
      if (!Array.isArray(item.artifacts) || item.artifacts.length === 0)
        errors.push(`${item.id}: verified evidence requires at least one artifact`);
    }

    for (const artifact of Array.isArray(item.artifacts) ? item.artifacts : []) {
      try {
        const absolute = safeArtifactPath(repoRoot, artifact.path);
        if (!statSync(absolute).isFile() || statSync(absolute).size === 0)
          errors.push(`${item.id}: artifact is empty: ${artifact.path}`);
        if (!shaPattern.test(artifact.sha256 ?? ""))
          errors.push(`${item.id}: artifact sha256 is invalid: ${artifact.path}`);
        else if (sha256(absolute) !== artifact.sha256)
          errors.push(`${item.id}: artifact sha256 mismatch: ${artifact.path}`);
      } catch (error) {
        errors.push(`${item.id}: ${error.message}`);
      }
    }

    if (item.status === "verified") {
      const parsedJSON = [];
      for (const artifact of item.artifacts.filter((candidate) =>
        candidate.path?.endsWith(".json"),
      )) {
        try {
          const absolute = safeArtifactPath(repoRoot, artifact.path);
          parsedJSON.push({ artifact, evidence: JSON.parse(readFileSync(absolute, "utf8")) });
        } catch (error) {
          errors.push(`${item.id}: invalid JSON ${artifact.path}: ${error.message}`);
        }
      }

      const expectedNames = expectedEvidenceNames(item.id);
      for (const { artifact, evidence } of parsedJSON) {
        if (!expectedNames.has(evidence?.name)) {
          errors.push(
            `${item.id}: ${artifact.path}: unregistered evidence profile ${evidence?.name ?? "<missing>"}`,
          );
        }
        if (releaseGitSHAValid && evidence?.git_sha !== releaseGitSHA) {
          errors.push(`${item.id}: ${artifact.path}: git_sha must match manifest release_git_sha`);
        }
        if (releaseGitSHAValid) {
          const finishedAt = Date.parse(evidence?.finished_at);
          const verifiedAt = Date.parse(item.verified_at);
          if (Number.isNaN(finishedAt) || Number.isNaN(verifiedAt) || verifiedAt < finishedAt) {
            errors.push(
              `${item.id}: ${artifact.path}: verified_at must be at or after the proof window`,
            );
          }
        }
      }

      const profile = (name) => parsedJSON.filter(({ evidence }) => evidence?.name === name);

      if (performanceExpectations[item.id]) {
        const records = profile(performanceExpectations[item.id].name);
        if (records.length !== 1)
          errors.push(`${item.id}: verified performance evidence requires exactly one JSON result`);
        for (const { artifact, evidence } of records)
          for (const error of validatePerformanceEvidence(item.id, evidence))
            errors.push(`${item.id}: ${artifact.path}: ${error}`);
      }

      if (item.id === "AURA-50.2") {
        const records = profile("auraedu-staging-two-school-isolation");
        if (records.length !== 1)
          errors.push(`${item.id}: verified isolation evidence requires exactly one JSON result`);
        for (const { artifact, evidence } of records)
          for (const error of validateIsolationEvidence(evidence))
            errors.push(`${item.id}: ${artifact.path}: ${error}`);
      }

      if (item.id === "AURA-18.9") {
        const records = profile("auraedu-staging-email-provider");
        if (records.length !== 1)
          errors.push(`${item.id}: verified provider evidence requires exactly one JSON result`);
        for (const { artifact, evidence } of records)
          for (const error of validateProviderEvidence(evidence))
            errors.push(`${item.id}: ${artifact.path}: ${error}`);
      }

      if (operationalExpectations[item.id]) {
        const records = profile(operationalExpectations[item.id].name);
        if (records.length !== 1)
          errors.push(`${item.id}: verified operational evidence requires exactly one JSON result`);
        for (const { artifact, evidence } of records)
          for (const error of validateOperationalEvidence(item.id, evidence))
            errors.push(`${item.id}: ${artifact.path}: ${error}`);
      }

      if (visualExpectations[item.id]) {
        const records = profile(visualExpectations[item.id].name);
        const pngArtifacts = item.artifacts.filter((artifact) => artifact.path?.endsWith(".png"));
        if (records.length !== 1)
          errors.push(`${item.id}: verified visual evidence requires exactly one JSON result`);
        const artifactPaths = new Set(item.artifacts.map((artifact) => artifact.path));
        for (const { artifact, evidence } of records) {
          for (const error of validateVisualEvidence(item.id, evidence, artifactPaths))
            errors.push(`${item.id}: ${artifact.path}: ${error}`);
          const referencedPNGs = new Set(
            (Array.isArray(evidence?.captures) ? evidence.captures : []).map(
              (capture) => capture?.screenshot,
            ),
          );
          for (const png of pngArtifacts) {
            if (!referencedPNGs.has(png.path))
              errors.push(`${item.id}: unreferenced PNG artifact: ${png.path}`);
          }
          for (const capture of Array.isArray(evidence?.captures) ? evidence.captures : []) {
            if (typeof capture?.screenshot !== "string" || !artifactPaths.has(capture.screenshot))
              continue;
            try {
              const dimensions = pngDimensions(safeArtifactPath(repoRoot, capture.screenshot));
              if (dimensions.width !== capture.width || dimensions.height < capture.height) {
                errors.push(
                  `${item.id}: ${capture.screenshot}: PNG dimensions do not match the declared viewport`,
                );
              }
            } catch (error) {
              errors.push(`${item.id}: ${capture.screenshot}: ${error.message}`);
            }
          }
        }
        const expectedPNGs =
          visualExpectations[item.id].states.length * visualExpectations[item.id].viewports.length;
        if (pngArtifacts.length !== expectedPNGs)
          errors.push(
            `${item.id}: verified visual evidence requires exactly ${expectedPNGs} PNG artifacts`,
          );
      }
    }
  }

  const ledgerIDs = new Set();
  for (const { id } of ledgerStatusRows(ledger)) {
    if (ledgerIDs.has(id)) errors.push(`${id}: duplicate story id in live ledger`);
    ledgerIDs.add(id);
  }
  const open = new Set(openLedgerIDs(ledger));
  for (const id of open)
    if (!pending.has(id)) errors.push(`${id}: ledger is unresolved but manifest is not pending`);
  for (const id of pending)
    if (!open.has(id)) errors.push(`${id}: manifest is pending but ledger is not unresolved`);
  if (assertReady && (pending.size > 0 || open.size > 0))
    errors.push(`production readiness is blocked by ${pending.size} pending evidence item(s)`);
  return errors;
}

function argumentValue(name, fallback) {
  const index = process.argv.indexOf(name);
  return index === -1 ? fallback : process.argv[index + 1];
}

const invokedPath = process.argv[1] ? resolve(process.argv[1]) : "";
if (invokedPath === fileURLToPath(import.meta.url)) {
  const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
  const manifestPath = resolve(
    repoRoot,
    argumentValue("--manifest", "release/evidence/manifest.json"),
  );
  const planPath = resolve(repoRoot, argumentValue("--plan", "agent_plan.md"));
  const assertReady = process.argv.includes("--assert-ready");
  let manifest;
  try {
    manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  } catch (error) {
    console.error(`release evidence: cannot read manifest: ${error.message}`);
    process.exit(1);
  }
  const ledger = readFileSync(planPath, "utf8");
  const errors = validateReadiness({ manifest, ledger, repoRoot, assertReady });
  if (errors.length > 0) {
    for (const error of errors) console.error(`release evidence: ${error}`);
    process.exit(1);
  }
  const pending = manifest.items.filter((item) => item.status === "pending").length;
  console.log(
    `Release evidence manifest valid: ${manifest.items.length} tracked, ${pending} pending.`,
  );
}
