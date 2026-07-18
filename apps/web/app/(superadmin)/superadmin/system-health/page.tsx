import { HeartPulse } from "lucide-react";
import { PageHeader, StatCard, DataTable } from "@auraedu/ui";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import { getCurrentTenantCode, getCurrentToken } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

// Probes must run on every request — never serve cached reachability.
export const dynamic = "force-dynamic";

type ProbeStatus = "reachable" | "degraded" | "unreachable";

interface ProbeDef {
  service: string;
  endpoint: string;
}

interface ProbeResult extends ProbeDef {
  status: ProbeStatus;
  detail: string;
  latencyMs: number;
}

// There are no dedicated per-service health endpoints yet, so reachability is derived
// by calling existing gateway-proxied endpoints server-side (all routed via the API
// gateway, which is itself covered: if it is down, every probe reports unreachable).
// TODO(follow-up): dedicated health routing (e.g. aggregated /healthz per service).
const PROBES: ProbeDef[] = [
  { service: "Tenant service", endpoint: "GET /api/v1/tenants?limit=1" },
  { service: "Feature flags (tenant service)", endpoint: "GET /api/v1/features" },
  { service: "Identity service", endpoint: "GET /api/v1/users?limit=1" },
  { service: "Billing service", endpoint: "GET /api/v1/billing/plans?limit=1" },
  { service: "Analytics service", endpoint: "GET /api/v1/analytics/kpis?limit=1" },
  { service: "Audit service", endpoint: "GET /api/v1/audit/logs?limit=1" },
];

const PROBE_TIMEOUT_MS = 5000;

async function probe(def: ProbeDef, token: string | undefined, tenantCode: string): Promise<ProbeResult> {
  const path = def.endpoint.replace(/^GET /, "");
  const started = Date.now();
  try {
    const res = await fetch(`${publicApiUrl}${path}`, {
      headers: {
        accept: "application/json",
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        ...(tenantCode ? { [tenantHeaderName]: tenantCode } : {}),
      },
      cache: "no-store",
      signal: AbortSignal.timeout(PROBE_TIMEOUT_MS),
    });
    const latencyMs = Date.now() - started;
    if (res.ok) {
      return { ...def, status: "reachable", detail: `HTTP ${res.status}`, latencyMs };
    }
    // The service responded but not successfully (authz, validation, or 5xx).
    return { ...def, status: "degraded", detail: `HTTP ${res.status}`, latencyMs };
  } catch (e) {
    return {
      ...def,
      status: "unreachable",
      detail: e instanceof Error ? e.message : "request failed",
      latencyMs: Date.now() - started,
    };
  }
}

function StatusPill({ status, detail }: { status: ProbeStatus; detail: string }) {
  const styles: Record<ProbeStatus, string> = {
    reachable: "bg-[var(--color-ok)]/10 text-[var(--color-ok)]",
    degraded: "bg-[var(--color-warn)]/10 text-[var(--color-warn)]",
    unreachable: "bg-[var(--color-crit)]/10 text-[var(--color-crit)]",
  };
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs capitalize ${styles[status]}`}>
      {status}
      <span className="ml-1.5 normal-case opacity-75">({detail})</span>
    </span>
  );
}

export default async function SystemHealthPage() {
  await requireAuth();

  const [token, tenantCode] = await Promise.all([getCurrentToken(), getCurrentTenantCode()]);
  const results = await Promise.all(PROBES.map((def) => probe(def, token, tenantCode)));
  const checkedAt = new Date();

  const reachable = results.filter((r) => r.status === "reachable").length;
  const degraded = results.filter((r) => r.status !== "reachable").length;
  const slowest = results.reduce((max, r) => Math.max(max, r.latencyMs), 0);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<HeartPulse className="size-7" />}
        title="System health"
        description="Live reachability of platform services, probed through the API gateway."
      />

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Services reachable"
          value={`${reachable}/${results.length}`}
          tone={degraded === 0 ? "ok" : "warn"}
        />
        <StatCard
          label="Degraded / unreachable"
          value={degraded}
          tone={degraded === 0 ? "default" : "warn"}
        />
        <StatCard label="Slowest probe" value={slowest} unit="ms" />
        <StatCard label="Checked at" value={checkedAt.toLocaleTimeString()} />
      </section>

      <DataTable
        caption="Service reachability"
        rows={results}
        keyExtractor={(r) => r.service}
        columns={[
          { key: "service", header: "Service", cell: (r) => r.service },
          {
            key: "endpoint",
            header: "Endpoint probed",
            cell: (r) => <span className="font-mono text-xs">{r.endpoint}</span>,
          },
          {
            key: "status",
            header: "Status",
            cell: (r) => <StatusPill status={r.status} detail={r.detail} />,
          },
          {
            key: "latency",
            header: "Latency",
            className: "w-24",
            cell: (r) => <span className="font-mono text-xs">{r.latencyMs} ms</span>,
          },
          {
            key: "checked",
            header: "Checked at",
            className: "w-32",
            cell: () => <span className="font-mono text-xs">{checkedAt.toLocaleTimeString()}</span>,
          },
        ]}
      />

      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <p className="text-sm text-[var(--muted-foreground)]">
          Status is inferred by probing existing gateway endpoints — a non-2xx response is
          reported as degraded even when the service itself is up (e.g. a missing permission
          returns HTTP 403). Dedicated per-service health routing is a follow-up.
        </p>
      </section>
    </div>
  );
}
