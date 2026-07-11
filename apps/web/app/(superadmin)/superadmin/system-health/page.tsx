import { HeartPulse, Activity, Server, Database } from "lucide-react";
import { PageHeader, StatCard } from "@auraedu/ui";

export default function SystemHealthPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<HeartPulse className="size-7" />}
        title="System health"
        description="Platform service status and operational metrics."
      />

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="API gateway" value="—" unit="healthy" tone="ok" />
        <StatCard label="Database" value="—" unit="connected" tone="ok" />
        <StatCard label="Event bus" value="—" unit="operational" />
        <StatCard label="Last deploy" value="—" unit="ago" />
      </section>

      <section className="grid gap-6 md:grid-cols-2">
        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="flex items-center gap-2 font-display font-semibold tracking-tight">
            <Activity className="size-4 text-[var(--primary)]" />
            Service metrics
          </h3>
          <p className="mt-4 text-sm text-[var(--muted-foreground)]">
            Live metrics will appear here once observability is wired.
          </p>
        </div>

        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="flex items-center gap-2 font-display font-semibold tracking-tight">
            <Server className="size-4 text-[var(--primary)]" />
            Upstream services
          </h3>
          <ul className="mt-4 space-y-2 text-sm text-[var(--muted-foreground)]">
            <li className="flex items-center justify-between">
              <span>API Gateway</span>
              <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs text-[var(--color-ok)]">
                Healthy
              </span>
            </li>
            <li className="flex items-center justify-between">
              <span>Tenant Service</span>
              <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs text-[var(--color-ok)]">
                Healthy
              </span>
            </li>
            <li className="flex items-center justify-between">
              <span>Billing Service</span>
              <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs text-[var(--color-ok)]">
                Healthy
              </span>
            </li>
            <li className="flex items-center justify-between">
              <span>Audit Service</span>
              <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs text-[var(--color-ok)]">
                Healthy
              </span>
            </li>
          </ul>
        </div>
      </section>

      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="flex items-center gap-2 font-display font-semibold tracking-tight">
          <Database className="size-4 text-[var(--primary)]" />
          Infrastructure overview
        </h3>
        <p className="mt-4 text-sm text-[var(--muted-foreground)]">
          Infrastructure status will appear here once health probes are integrated.
        </p>
      </section>
    </div>
  );
}
