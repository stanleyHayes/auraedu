import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

import {
  BLOCKED_PENALTY,
  WATCH_PENALTY,
  criticalAuditEvents,
  deploymentGate,
  onboardingReviewGate,
  operationsHealth,
  platformHealthGate,
  providerConfigGates,
  suspendedTenantsGate,
} from "../lib/superadmin-readiness.ts";
import type { PlatformHealthReport } from "../lib/system-health.ts";

function source(path: string) {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
}

const readinessPage = "../app/(superadmin)/superadmin/readiness/page.tsx";

void test("launch readiness route exists with the console hierarchy and live recompute", () => {
  const page = source(readinessPage);
  assert.match(page, /<PageHeader\b/);
  assert.match(page, /title="Launch readiness"/);
  assert.match(page, /requireAuth\(\)/);
  assert.match(
    page,
    /export const dynamic = "force-dynamic"/,
    "gates must never serve a cached snapshot",
  );
  assert.match(page, /<Reveal\b/, "reveals keep the page reduced-motion safe");
});

void test("readiness page pulls every live signal from the documented endpoints", () => {
  const page = source(readinessPage);
  assert.match(page, /\/api\/v1\/platform\/health/);
  assert.match(page, /\/api\/v1\/super-admin\/onboarding-requests/);
  assert.match(page, /\/api\/v1\/tenants\?limit=100/);
  assert.match(page, /\/api\/v1\/audit\/logs/);
  assert.match(
    page,
    /createServerClient\(\)/,
    "audit feed is read tenantless, without a tenant pin",
  );
  assert.doesNotMatch(page, /createServerClientForTenant/);
});

void test("env gates are read server-side and never print secret values", () => {
  const lib = source("../lib/superadmin-readiness.ts");
  const page = source(readinessPage);

  for (const name of [
    "RESEND_API_KEY",
    "TWILIO_ACCOUNT_SID",
    "PAYSTACK_SECRET_KEY",
    "AURAEDU_LEGAL_REVIEW_CONFIRMED",
    "AURAEDU_GROWTH_POLICY_CONFIRMED",
    "AURAEDU_UAT_SIGNOFF_CONFIRMED",
    "NEXT_PUBLIC_API_URL",
    "AURAEDU_API_URL",
  ]) {
    assert.match(lib, new RegExp(name), `gate for ${name} is missing`);
  }
  assert.match(lib, /process\.env/, "env is read on the server");
  assert.doesNotMatch(page, /process\.env/, "the page renders gate results, not env reads");
  assert.match(lib, /is configured\./);
  assert.match(lib, /is missing\./);
  assert.doesNotMatch(
    lib,
    /detail:[^,]*\$\{raw\}/,
    "raw env values must never be interpolated into gate output",
  );
});

void test("provider and sign-off env gates map to honest statuses", () => {
  const configured = providerConfigGates({
    RESEND_API_KEY: "re_test",
    TWILIO_ACCOUNT_SID: "AC_test",
    PAYSTACK_SECRET_KEY: "sk_test",
    AURAEDU_LEGAL_REVIEW_CONFIRMED: "true",
    AURAEDU_GROWTH_POLICY_CONFIRMED: "pending",
  });
  const byId = new Map(configured.map((g) => [g.id, g]));
  const gate = (id: string) => {
    const found = byId.get(id);
    assert.ok(found, `gate ${id} must exist`);
    return found;
  };

  assert.equal(gate("provider-email").status, "ready");
  assert.equal(gate("provider-email").detail, "RESEND_API_KEY is configured.");
  assert.equal(gate("provider-messaging").status, "ready");
  assert.equal(gate("provider-payments").status, "ready");
  assert.equal(gate("signoff-legal").status, "ready");
  assert.equal(gate("signoff-growth").status, "watch", "set-but-not-true is watch");
  assert.equal(gate("signoff-uat").status, "blocked", "missing sign-off is blocked");

  const missing = providerConfigGates({});
  assert.ok(
    missing.every((g) => g.status === "blocked"),
    "an empty environment blocks every provider and sign-off gate",
  );
  assert.ok(
    missing.every((g) => !g.detail.includes("re_test") && !g.detail.includes("sk_test")),
    "details report configured/missing, never values",
  );
});

void test("deployment gate requires a configured, non-loopback gateway origin", () => {
  assert.equal(deploymentGate({ NEXT_PUBLIC_API_URL: "https://api.auraedu.com" }).status, "ready");
  assert.equal(deploymentGate({ AURAEDU_API_URL: "https://gateway.internal.run" }).status, "ready");
  assert.equal(deploymentGate({ NEXT_PUBLIC_API_URL: "http://localhost:8080" }).status, "blocked");
  assert.equal(deploymentGate({ NEXT_PUBLIC_API_URL: "http://127.0.0.1:8080" }).status, "blocked");
  assert.equal(deploymentGate({}).status, "blocked");
  assert.match(deploymentGate({}).remedy, /Gateway origin|gateway origin/i);
});

void test("live gates map platform health, onboarding, and tenant signals honestly", () => {
  const healthy: PlatformHealthReport = {
    status: "healthy",
    generated_at: "2026-07-28T08:00:00Z",
    checks: [
      {
        service: "student-service",
        endpoint: "/ready",
        status: "healthy",
        detail: "ready",
        latency_ms: 9,
      },
    ],
  };
  assert.equal(platformHealthGate(healthy).status, "ready");
  assert.equal(platformHealthGate({ ...healthy, status: "degraded" }).status, "watch");
  assert.equal(platformHealthGate({ ...healthy, status: "down" }).status, "blocked");
  assert.equal(platformHealthGate(null).status, "blocked", "an unreachable report blocks launch");

  assert.equal(onboardingReviewGate(0).status, "ready");
  assert.equal(onboardingReviewGate(2).status, "watch");
  assert.equal(onboardingReviewGate(null).status, "watch", "unverifiable is never reported ready");

  assert.equal(suspendedTenantsGate(0).status, "ready");
  assert.equal(suspendedTenantsGate(1).status, "watch");
  assert.equal(suspendedTenantsGate(null).status, "watch");
});

void test("operations health scores 100 − 15 per blocked − 7 per watch with bands", () => {
  assert.equal(BLOCKED_PENALTY, 15);
  assert.equal(WATCH_PENALTY, 7);

  const allReady = operationsHealth(
    providerConfigGates({
      RESEND_API_KEY: "x",
      TWILIO_ACCOUNT_SID: "x",
      PAYSTACK_SECRET_KEY: "x",
      AURAEDU_LEGAL_REVIEW_CONFIRMED: "true",
      AURAEDU_GROWTH_POLICY_CONFIRMED: "true",
      AURAEDU_UAT_SIGNOFF_CONFIRMED: "true",
    }),
  );
  assert.equal(allReady.score, 100);
  assert.equal(allReady.band, "Healthy");
  assert.equal(allReady.signals.length, 0);

  const mixed = operationsHealth([
    platformHealthGate(null),
    onboardingReviewGate(3),
    suspendedTenantsGate(0),
  ]);
  assert.equal(mixed.score, 100 - 15 - 7);
  assert.equal(mixed.band, "Attention");
  assert.deepEqual(
    mixed.signals.map((s) => s.weight),
    [-15, -7],
    "contributing signals carry their deduction",
  );

  const critical = operationsHealth([
    platformHealthGate(null),
    deploymentGate({}),
    onboardingReviewGate(null),
    suspendedTenantsGate(2),
  ]);
  assert.equal(critical.score, 100 - 30 - 14);
  assert.equal(critical.band, "Critical");

  const floor = operationsHealth(providerConfigGates({}));
  assert.equal(floor.score >= 0, true, "the score never goes negative");
});

void test("critical events keep only security-relevant audit entries", () => {
  const events = criticalAuditEvents([
    { id: "1", event_type: "auth.login_failed" },
    { id: "2", event_type: "student.created" },
    { id: "3", event_type: "security.policy_updated" },
    { id: "4", event_type: "tenant.suspended" },
    { id: "5", event_type: "billing.invoice_paid" },
    { id: "6" },
  ]);
  assert.deepEqual(
    events.map((e) => e.id),
    ["1", "3", "4"],
  );
});

void test("readiness page renders score, gate groups, and honest empty/unavailable states", () => {
  const page = source(readinessPage);
  assert.match(page, /Operations Health/);
  assert.match(page, /Healthy/);
  assert.match(page, /Attention/);
  assert.match(page, /Critical/);
  assert.match(page, /Live platform signals/);
  assert.match(page, /Configuration &amp; sign-offs/);
  assert.match(page, /Recent critical events/);
  assert.match(page, /<EmptyState\b/, "empty and unavailable states must be designed");
  assert.match(page, /Critical events unavailable/);
  assert.match(page, /No critical events/);
  assert.match(page, /var\(--color-ok\)/);
  assert.match(page, /var\(--color-warn\)/);
  assert.match(page, /var\(--color-crit\)/, "blocked pills use the critical token");
});

void test("readiness route ships a loading skeleton and an error boundary", () => {
  const loading = source("../app/(superadmin)/superadmin/readiness/loading.tsx");
  const error = source("../app/(superadmin)/superadmin/readiness/error.tsx");
  assert.match(loading, /PortalRouteLoading/);
  assert.match(error, /PortalRouteError/);
});

void test("superadmin navigation links launch readiness from the overview group", () => {
  const tenant = source("../lib/tenant.ts");
  const overview =
    /heading: "Overview",\s*items: \[\s*\{ label: "Dashboard", href: "\/superadmin" \},[\s\S]*?\]/.exec(
      tenant,
    );
  assert.ok(overview, "the superadmin overview group must exist");
  assert.match(overview[0], /\{ label: "Launch readiness", href: "\/superadmin\/readiness" \}/);
});
