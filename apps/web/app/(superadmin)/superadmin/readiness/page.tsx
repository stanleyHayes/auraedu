import { CircleAlert, Gauge, Rocket, ShieldCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader, Reveal, Watermark } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { formatDateTime } from "@/lib/superadmin-drilldown";
import type { PlatformHealthReport } from "@/lib/system-health";
import {
  criticalAuditEvents,
  deploymentGate,
  onboardingReviewGate,
  operationsHealth,
  platformHealthGate,
  providerConfigGates,
  suspendedTenantsGate,
  type GateStatus,
  type OperationsBand,
  type ReadinessGate,
} from "@/lib/superadmin-readiness";

type Tenant = OpenAPI.tenant_v1.components["schemas"]["Tenant"];
type OnboardingRequest = OpenAPI.tenant_v1.components["schemas"]["OnboardingRequest"];
type AuditLog = OpenAPI.audit_v1.components["schemas"]["AuditLog"];

// Launch readiness is computed per request; a cached snapshot would lie to the operator.
export const dynamic = "force-dynamic";

const STATUS_PILL: Record<GateStatus, string> = {
  ready: "bg-[var(--color-ok)]/10 text-[var(--color-ok)]",
  watch: "bg-[var(--color-warn)]/10 text-[var(--color-warn)]",
  blocked: "bg-[var(--color-crit)]/10 text-[var(--color-crit)]",
};

const BAND_STYLE: Record<OperationsBand, string> = {
  Healthy: "border-[var(--color-ok)]/30 bg-[var(--color-ok)]/5",
  Attention: "border-[var(--color-warn)]/30 bg-[var(--color-warn)]/5",
  Critical: "border-[var(--color-crit)]/30 bg-[var(--color-crit)]/5",
};

function GateStatusPill({ status }: { status: GateStatus }) {
  return (
    <span
      className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${STATUS_PILL[status]}`}
    >
      {status}
    </span>
  );
}

function GateCard({ gate }: { gate: ReadinessGate }) {
  return (
    <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
      <div className="flex items-start justify-between gap-3">
        <h4 className="font-sans font-semibold tracking-tight">{gate.title}</h4>
        <GateStatusPill status={gate.status} />
      </div>
      <p className="mt-2 text-sm leading-6 text-[var(--muted-foreground)]">{gate.checks}</p>
      <p className="mt-3 text-sm font-medium">{gate.detail}</p>
      {gate.status !== "ready" ? (
        <p className="mt-3 rounded-[var(--radius-sm)] bg-[var(--muted)]/60 p-3 text-sm leading-6 text-[var(--muted-foreground)]">
          {gate.remedy}
        </p>
      ) : null}
    </div>
  );
}

async function loadHealth(): Promise<PlatformHealthReport | null> {
  try {
    const client = await createServerClient();
    return await client.get<PlatformHealthReport>("/api/v1/platform/health");
  } catch {
    return null;
  }
}

async function loadPendingOnboarding(): Promise<number | null> {
  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: OnboardingRequest[] }>(
      "/api/v1/super-admin/onboarding-requests?limit=25",
    );
    return (res.data ?? []).filter((r) => r.status === "pending_review").length;
  } catch {
    return null;
  }
}

async function loadSuspendedTenants(): Promise<number | null> {
  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: Tenant[] }>("/api/v1/tenants?limit=100");
    return (res.data ?? []).filter((t) => t.status === "suspended").length;
  } catch {
    return null;
  }
}

/** Tenantless platform read — the audit route serves the cross-tenant feed without a pin. */
async function loadCriticalEvents(): Promise<{ events: AuditLog[] } | { error: string }> {
  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: AuditLog[] }>("/api/v1/audit/logs?limit=25");
    return { events: criticalAuditEvents(res.data ?? []) };
  } catch (cause) {
    return {
      error: cause instanceof Error ? cause.message : "The audit feed could not be loaded.",
    };
  }
}

export default async function LaunchReadinessPage() {
  await requireAuth();

  const [health, pendingOnboarding, suspendedTenants, audit] = await Promise.all([
    loadHealth(),
    loadPendingOnboarding(),
    loadSuspendedTenants(),
    loadCriticalEvents(),
  ]);

  const liveGates = [
    platformHealthGate(health),
    onboardingReviewGate(pendingOnboarding),
    suspendedTenantsGate(suspendedTenants),
  ];
  const configGates = [deploymentGate(), ...providerConfigGates()];
  const ops = operationsHealth([...liveGates, ...configGates]);

  return (
    <div className="relative space-y-6">
      <Watermark className="pointer-events-none absolute -right-6 -top-10 text-[10rem] opacity-[0.03]">
        Launch
      </Watermark>
      <Reveal>
        <PageHeader
          icon={<Rocket className="size-7" />}
          title="Launch readiness"
          description="The launch-morning gate check: provider configuration, human sign-offs, and live platform signals, scored into one Operations Health number."
        />
      </Reveal>

      <Reveal delay={40}>
        <section
          className={`flex flex-col gap-5 rounded-2xl border p-6 sm:flex-row sm:items-center sm:justify-between ${BAND_STYLE[ops.band]}`}
        >
          <div className="flex items-start gap-4">
            {ops.band === "Healthy" ? (
              <ShieldCheck className="mt-1 size-8 text-[var(--color-ok)]" />
            ) : (
              <CircleAlert
                className={`mt-1 size-8 ${
                  ops.band === "Critical" ? "text-[var(--color-crit)]" : "text-[var(--color-warn)]"
                }`}
              />
            )}
            <div>
              <p className="font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--muted-foreground)]">
                Operations Health
              </p>
              <p className="mt-1 text-5xl font-black tracking-[-0.035em]">
                {ops.score}
                <span className="text-lg font-bold text-[var(--muted-foreground)]"> / 100</span>
              </p>
              <p className="mt-2 text-sm text-[var(--muted-foreground)]">
                {ops.ready} ready · {ops.watch} watch · {ops.blocked} blocked across{" "}
                {ops.ready + ops.watch + ops.blocked} gates.
              </p>
            </div>
          </div>
          <div className="sm:max-w-xs sm:text-right">
            <span
              className={`inline-block rounded-full px-3 py-1 text-sm font-semibold ${
                ops.band === "Healthy"
                  ? "bg-[var(--color-ok)]/10 text-[var(--color-ok)]"
                  : ops.band === "Attention"
                    ? "bg-[var(--color-warn)]/10 text-[var(--color-warn)]"
                    : "bg-[var(--color-crit)]/10 text-[var(--color-crit)]"
              }`}
            >
              {ops.band}
            </span>
            {ops.signals.length > 0 ? (
              <ul className="mt-3 space-y-1 text-sm text-[var(--muted-foreground)]">
                {ops.signals.map((signal) => (
                  <li key={signal.label} className="flex items-center justify-between gap-4">
                    <span>{signal.label}</span>
                    <span
                      className={`font-mono text-xs font-semibold ${
                        signal.status === "blocked"
                          ? "text-[var(--color-crit)]"
                          : "text-[var(--color-warn)]"
                      }`}
                    >
                      {signal.weight}
                    </span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-3 text-sm text-[var(--muted-foreground)]">
                Every gate is ready — no deductions.
              </p>
            )}
          </div>
        </section>
      </Reveal>

      <Reveal delay={80}>
        <section className="space-y-3">
          <h3 className="font-sans font-semibold tracking-tight">Live platform signals</h3>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {liveGates.map((gate) => (
              <GateCard key={gate.id} gate={gate} />
            ))}
          </div>
        </section>
      </Reveal>

      <Reveal delay={120}>
        <section className="space-y-3">
          <h3 className="font-sans font-semibold tracking-tight">Configuration &amp; sign-offs</h3>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {configGates.map((gate) => (
              <GateCard key={gate.id} gate={gate} />
            ))}
          </div>
        </section>
      </Reveal>

      <Reveal delay={160}>
        <section className="space-y-3">
          <div className="flex items-center gap-2">
            <Gauge className="size-4 text-[var(--primary)]" />
            <h3 className="font-sans font-semibold tracking-tight">Recent critical events</h3>
          </div>
          {"error" in audit ? (
            <EmptyState
              title="Critical events unavailable"
              description={audit.error}
              icon={<CircleAlert className="size-8" />}
            />
          ) : audit.events.length === 0 ? (
            <EmptyState
              title="No critical events"
              description="No security-relevant audit events across tenants in the recent window."
              icon={<ShieldCheck className="size-8" />}
            />
          ) : (
            <DataTable
              caption="Recent security-relevant audit events across all tenants"
              rows={audit.events}
              keyExtractor={(log) => log.id}
              columns={[
                {
                  key: "time",
                  header: "Occurred",
                  cell: (log) => (
                    <span className="font-mono text-xs">{formatDateTime(log.occurred_at)}</span>
                  ),
                },
                {
                  key: "event",
                  header: "Event type",
                  cell: (log) => <span className="font-mono text-xs">{log.event_type}</span>,
                },
                {
                  key: "tenant",
                  header: "Tenant",
                  cell: (log) => <span className="font-mono text-xs">{log.tenant_id}</span>,
                },
                {
                  key: "actor",
                  header: "Actor",
                  cell: (log) => (
                    <span className="font-mono text-xs">{log.actor_id ?? "system"}</span>
                  ),
                },
              ]}
            />
          )}
        </section>
      </Reveal>
    </div>
  );
}
