import { CircleAlert, HeartPulse, ShieldCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import {
  summarizePlatformHealth,
  type DependencyHealthCheck,
  type DependencyStatus,
  type PlatformHealthReport,
} from "@/lib/system-health";

// Operators must always see current readiness, never a cached deployment snapshot.
export const dynamic = "force-dynamic";

function StatusPill({ status, detail }: { status: DependencyStatus; detail: string }) {
  const styles: Record<DependencyStatus, string> = {
    healthy: "bg-[var(--color-ok)]/10 text-[var(--color-ok)]",
    degraded: "bg-[var(--color-warn)]/10 text-[var(--color-warn)]",
    unreachable: "bg-[var(--color-crit)]/10 text-[var(--color-crit)]",
  };
  return (
    <span className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${styles[status]}`}>
      {status}
      <span className="ml-1.5 normal-case opacity-75">· {detail}</span>
    </span>
  );
}

export default async function SystemHealthPage() {
  await requireAuth();
  const client = await createServerClient();
  let report: PlatformHealthReport | null = null;
  let error: string | null = null;
  try {
    report = await client.get<PlatformHealthReport>("/api/v1/platform/health");
  } catch (cause) {
    error =
      cause instanceof Error ? cause.message : "The platform health report could not be loaded.";
  }

  const summary = report ? summarizePlatformHealth(report) : null;
  const generatedAt = report ? new Date(report.generated_at) : null;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<HeartPulse className="size-7" />}
        title="System health"
        description="Live dependency readiness reported by the API gateway, including database-backed checks where services expose them."
      />

      {report && summary ? (
        <>
          <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              label="Services healthy"
              value={`${summary.healthy}/${report.checks.length}`}
              tone={report.status === "healthy" ? "ok" : "warn"}
            />
            <StatCard
              label="Degraded"
              value={summary.degraded}
              tone={summary.degraded === 0 ? "default" : "warn"}
            />
            <StatCard
              label="Unreachable"
              value={summary.unreachable}
              tone={summary.unreachable === 0 ? "default" : "warn"}
            />
            <StatCard label="Slowest readiness probe" value={summary.slowestMs} unit="ms" />
          </section>

          <section
            className={`flex flex-col gap-3 rounded-2xl border p-5 sm:flex-row sm:items-center sm:justify-between ${
              report.status === "healthy"
                ? "border-emerald-200 bg-emerald-50/70"
                : "border-amber-200 bg-amber-50/70"
            }`}
          >
            <div className="flex items-start gap-3">
              {report.status === "healthy" ? (
                <ShieldCheck className="mt-0.5 size-5 text-emerald-700" />
              ) : (
                <CircleAlert className="mt-0.5 size-5 text-amber-700" />
              )}
              <div>
                <p className="font-bold capitalize">Platform dependencies {report.status}</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  A degraded service answered its readiness endpoint with a non-success status. An
                  unreachable service did not answer before the bounded timeout.
                </p>
              </div>
            </div>
            <p className="shrink-0 text-xs font-semibold text-muted-foreground">
              Checked {generatedAt?.toLocaleString("en-GB")}
            </p>
          </section>

          <DataTable
            caption="Private service readiness"
            rows={report.checks}
            keyExtractor={(check) => check.service}
            columns={[
              {
                key: "service",
                header: "Service",
                cell: (check: DependencyHealthCheck) => (
                  <span className="font-semibold">{check.service.replaceAll("-", " ")}</span>
                ),
              },
              {
                key: "endpoint",
                header: "Probe",
                cell: (check: DependencyHealthCheck) => (
                  <span className="font-mono text-xs">{check.endpoint}</span>
                ),
              },
              {
                key: "status",
                header: "Readiness",
                cell: (check: DependencyHealthCheck) => (
                  <StatusPill status={check.status} detail={check.detail} />
                ),
              },
              {
                key: "latency",
                header: "Latency",
                className: "w-28",
                cell: (check: DependencyHealthCheck) => (
                  <span className="font-mono text-xs">{check.latency_ms} ms</span>
                ),
              },
            ]}
          />

          <p className="rounded-xl border border-border bg-muted/40 p-4 text-sm leading-6 text-muted-foreground">
            Private hostnames and transport errors are intentionally redacted. Python AI services
            expose <code>/health</code>; all database-backed Go services report <code>/ready</code>{" "}
            with their PostgreSQL dependency check.
          </p>
        </>
      ) : (
        <EmptyState
          title="System health is unavailable"
          description={error ?? "The gateway returned no dependency report."}
          icon={<CircleAlert className="size-8" />}
        />
      )}
    </div>
  );
}
