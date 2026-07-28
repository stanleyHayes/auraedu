/**
 * Launch-readiness gates and the Operations Health score behind
 * app/(superadmin)/superadmin/readiness. All status mapping lives here as pure
 * functions so the page stays a thin fetch-and-render shell and the gate logic
 * is unit-testable. Provider and sign-off gates read server env only and report
 * configured/missing — secret values are never interpolated into output.
 */

import type { PlatformHealthReport } from "./system-health";

export type GateStatus = "ready" | "watch" | "blocked";

export interface ReadinessGate {
  id: string;
  title: string;
  /** What the gate verifies. */
  checks: string;
  status: GateStatus;
  /** Honest current reading — configured/missing or a count, never a secret value. */
  detail: string;
  /** What the operator does next when the gate is not ready. */
  remedy: string;
}

export type GateEnv = Record<string, string | undefined>;

function envValue(env: GateEnv, name: string): string | undefined {
  const value = env[name]?.trim();
  return value === "" ? undefined : value;
}

/** Provider key present → ready; absent → blocked. Values are never echoed. */
function providerGate(
  env: GateEnv,
  envName: string,
  id: string,
  title: string,
  checks: string,
  remedy: string,
): ReadinessGate {
  const configured = envValue(env, envName) !== undefined;
  return {
    id,
    title,
    checks,
    status: configured ? "ready" : "blocked",
    detail: configured ? `${envName} is configured.` : `${envName} is missing.`,
    remedy,
  };
}

/**
 * Human sign-off tri-state: the string "true" → ready, set to anything else →
 * watch (someone started but did not confirm), missing entirely → blocked.
 */
function signoffGate(
  env: GateEnv,
  envName: string,
  id: string,
  title: string,
  checks: string,
  remedy: string,
): ReadinessGate {
  const raw = envValue(env, envName);
  if (raw === "true") {
    return { id, title, checks, status: "ready", detail: `${envName} is confirmed.`, remedy };
  }
  if (raw !== undefined) {
    return {
      id,
      title,
      checks,
      status: "watch",
      detail: `${envName} is set but not confirmed.`,
      remedy,
    };
  }
  return { id, title, checks, status: "blocked", detail: `${envName} is missing.`, remedy };
}

const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "0.0.0.0", "::1", "[::1]"]);

function isLoopbackUrl(raw: string): boolean {
  try {
    return LOOPBACK_HOSTS.has(new URL(raw).hostname.toLowerCase());
  } catch {
    return true;
  }
}

/**
 * The known Vercel blocker: the public Gateway origin must be configured and
 * must not point at a loopback address, or browser traffic never leaves the
 * deployment that serves it.
 */
export function deploymentGate(env: GateEnv = process.env): ReadinessGate {
  const raw = envValue(env, "NEXT_PUBLIC_API_URL") ?? envValue(env, "AURAEDU_API_URL");
  const ready = raw !== undefined && !isLoopbackUrl(raw);
  return {
    id: "deployment-api-origin",
    title: "Gateway origin",
    checks:
      "Verifies NEXT_PUBLIC_API_URL or AURAEDU_API_URL is configured and points at a public API gateway origin, not a loopback address.",
    status: ready ? "ready" : "blocked",
    detail: ready
      ? "A public Gateway origin is configured."
      : "No public Gateway origin is configured.",
    remedy:
      "Set NEXT_PUBLIC_API_URL (or AURAEDU_API_URL) to the deployed API gateway origin in the hosting environment and redeploy — a loopback or missing value is the known Vercel launch blocker.",
  };
}

/** Provider and human sign-off gates, computed from server env without exposing values. */
export function providerConfigGates(env: GateEnv = process.env): ReadinessGate[] {
  return [
    providerGate(
      env,
      "RESEND_API_KEY",
      "provider-email",
      "Email delivery",
      "Verifies transactional email (invites, receipts, alerts) can be sent through the configured provider.",
      "Provision the Resend API key and set RESEND_API_KEY in the server environment, then re-check this page.",
    ),
    providerGate(
      env,
      "TWILIO_ACCOUNT_SID",
      "provider-messaging",
      "SMS / WhatsApp delivery",
      "Verifies SMS and WhatsApp notifications (OTP, guardian alerts) can be delivered through the configured provider.",
      "Provision the Twilio credentials and set TWILIO_ACCOUNT_SID in the server environment, then re-check this page.",
    ),
    providerGate(
      env,
      "PAYSTACK_SECRET_KEY",
      "provider-payments",
      "Payments",
      "Verifies school fee collections and subscription billing can charge through the configured provider.",
      "Provision the Paystack secret key and set PAYSTACK_SECRET_KEY in the server environment, then re-check this page.",
    ),
    signoffGate(
      env,
      "AURAEDU_LEGAL_REVIEW_CONFIRMED",
      "signoff-legal",
      "Legal review sign-off",
      "Verifies the legal review of launch terms, policies, and data processing has been formally confirmed.",
      "Complete the legal review and set AURAEDU_LEGAL_REVIEW_CONFIRMED=true once signed off.",
    ),
    signoffGate(
      env,
      "AURAEDU_GROWTH_POLICY_CONFIRMED",
      "signoff-growth",
      "Growth policy sign-off",
      "Verifies the growth and communications policy (consent, messaging cadence) has been formally confirmed.",
      "Complete the growth policy review and set AURAEDU_GROWTH_POLICY_CONFIRMED=true once signed off.",
    ),
    signoffGate(
      env,
      "AURAEDU_UAT_SIGNOFF_CONFIRMED",
      "signoff-uat",
      "UAT sign-off",
      "Verifies user acceptance testing passed and the go/no-go was formally confirmed.",
      "Finish UAT, resolve outstanding defects, and set AURAEDU_UAT_SIGNOFF_CONFIRMED=true once signed off.",
    ),
  ];
}

/** All services healthy → ready; degraded → watch; down or unreachable → blocked. */
export function platformHealthGate(report: PlatformHealthReport | null): ReadinessGate {
  const checks =
    "Verifies every private service answers its readiness probe through GET /api/v1/platform/health.";
  const remedy =
    "Open System health, identify the degraded or unreachable services, and restore them before launch.";
  if (!report) {
    return {
      id: "live-platform-health",
      title: "Platform services",
      checks,
      status: "blocked",
      detail: "The platform health report is unreachable.",
      remedy,
    };
  }
  if (report.status === "healthy") {
    return {
      id: "live-platform-health",
      title: "Platform services",
      checks,
      status: "ready",
      detail: `All ${report.checks.length} services report healthy.`,
      remedy,
    };
  }
  if (report.status === "degraded") {
    const degraded = report.checks.filter((c) => c.status !== "healthy").length;
    return {
      id: "live-platform-health",
      title: "Platform services",
      checks,
      status: "watch",
      detail: `${degraded} of ${report.checks.length} services are degraded.`,
      remedy,
    };
  }
  return {
    id: "live-platform-health",
    title: "Platform services",
    checks,
    status: "blocked",
    detail: "The platform reports down.",
    remedy,
  };
}

/** Pending onboarding reviews are not a launch blocker, but they are operator debt → watch. */
export function onboardingReviewGate(pending: number | null): ReadinessGate {
  const checks =
    "Verifies no school onboarding requests are waiting for review via GET /api/v1/super-admin/onboarding-requests.";
  const remedy = "Open Onboarding and approve or reject the waiting requests.";
  if (pending === null) {
    return {
      id: "live-onboarding-pending",
      title: "Onboarding queue",
      checks,
      status: "watch",
      detail: "The onboarding queue could not be verified.",
      remedy,
    };
  }
  if (pending > 0) {
    return {
      id: "live-onboarding-pending",
      title: "Onboarding queue",
      checks,
      status: "watch",
      detail: `${pending} request${pending === 1 ? "" : "s"} pending review.`,
      remedy,
    };
  }
  return {
    id: "live-onboarding-pending",
    title: "Onboarding queue",
    checks,
    status: "ready",
    detail: "No requests pending review.",
    remedy,
  };
}

/** A suspended tenant signals billing or trust-and-safety action in flight → watch. */
export function suspendedTenantsGate(suspended: number | null): ReadinessGate {
  const checks = "Verifies no tenant is currently suspended via GET /api/v1/tenants.";
  const remedy =
    "Open Tenants, review each suspended school, and resolve or document the suspension.";
  if (suspended === null) {
    return {
      id: "live-tenants-suspended",
      title: "Suspended tenants",
      checks,
      status: "watch",
      detail: "Tenant status could not be verified.",
      remedy,
    };
  }
  if (suspended > 0) {
    return {
      id: "live-tenants-suspended",
      title: "Suspended tenants",
      checks,
      status: "watch",
      detail: `${suspended} tenant${suspended === 1 ? "" : "s"} suspended.`,
      remedy,
    };
  }
  return {
    id: "live-tenants-suspended",
    title: "Suspended tenants",
    checks,
    status: "ready",
    detail: "No tenants suspended.",
    remedy,
  };
}

export const BLOCKED_PENALTY = 15;
export const WATCH_PENALTY = 7;

export type OperationsBand = "Healthy" | "Attention" | "Critical";

export interface OperationsSignal {
  label: string;
  status: GateStatus;
  weight: number;
}

export interface OperationsHealth {
  score: number;
  band: OperationsBand;
  ready: number;
  watch: number;
  blocked: number;
  signals: OperationsSignal[];
}

/** 100 − 15 per blocked gate − 7 per watch gate, floored at 0. */
export function operationsHealth(gates: ReadinessGate[]): OperationsHealth {
  const blocked = gates.filter((g) => g.status === "blocked").length;
  const watch = gates.filter((g) => g.status === "watch").length;
  const ready = gates.length - blocked - watch;
  const score = Math.max(0, 100 - BLOCKED_PENALTY * blocked - WATCH_PENALTY * watch);
  const band: OperationsBand = score >= 90 ? "Healthy" : score >= 60 ? "Attention" : "Critical";
  const signals = gates
    .filter((g) => g.status !== "ready")
    .map((g) => ({
      label: g.title,
      status: g.status,
      weight: g.status === "blocked" ? -BLOCKED_PENALTY : -WATCH_PENALTY,
    }));
  return { score, band, ready, watch, blocked, signals };
}

export interface AuditEventLike {
  id: string;
  event_type?: string | null;
  tenant_id?: string | null;
  actor_id?: string | null;
  occurred_at?: string | null;
}

/** Event types an operator should see on launch morning. */
const CRITICAL_EVENT_PATTERN =
  /security|breach|lockout|login|auth|mfa|token|permission|policy|suspend/i;

/**
 * Keep only security-relevant audit events. Filtering is client-side on
 * event_type; events without a type are excluded rather than guessed about.
 */
export function criticalAuditEvents<T extends AuditEventLike>(logs: T[], limit = 8): T[] {
  return logs
    .filter(
      (log) => typeof log.event_type === "string" && CRITICAL_EVENT_PATTERN.test(log.event_type),
    )
    .slice(0, limit);
}
