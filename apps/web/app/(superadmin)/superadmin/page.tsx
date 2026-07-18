import Link from "next/link";
import { Building2, CreditCard, Receipt, ScrollText, HeartPulse } from "lucide-react";
import { StatCard, Button, Reveal, Watermark } from "@auraedu/ui";
import { createGatewayClient } from "@auraedu/api-client";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient, getCurrentToken } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Tenant = OpenAPI.tenant_v1.components["schemas"]["Tenant"];

interface PlatformStats {
  activeTenants: number | null;
  activeSubscriptions: number | null;
  students: number | null;
  staff: number | null;
}

async function loadPlatformStats(): Promise<PlatformStats> {
  const stats: PlatformStats = {
    activeTenants: null,
    activeSubscriptions: null,
    students: null,
    staff: null,
  };

  let tenants: Tenant[];
  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: Tenant[] }>("/api/v1/tenants?limit=50");
    tenants = res.data ?? [];
    stats.activeTenants = tenants.filter((t) => t.status === "active").length;
  } catch {
    return stats;
  }

  // Billing/student/staff endpoints are tenant-scoped, so aggregate per tenant.
  const token = await getCurrentToken();
  let subscriptions = 0;
  let students = 0;
  let staff = 0;
  let sawSubscriptions = false;
  let sawStudents = false;
  let sawStaff = false;

  await Promise.all(
    tenants.map(async (t) => {
      const client = createGatewayClient({
        baseUrl: publicApiUrl,
        tenantHeader: tenantHeaderName,
        getToken: () => token,
        getTenantCode: () => t.tenant_code,
      });
      const [subs, studs, staffRes] = await Promise.allSettled([
        client.get<{ data?: { status?: string }[] }>("/api/v1/billing/subscriptions?limit=100"),
        client.get<{ data?: unknown[] }>("/api/v1/students?limit=100"),
        client.get<{ data?: unknown[] }>("/api/v1/staff?limit=100"),
      ]);
      if (subs.status === "fulfilled") {
        sawSubscriptions = true;
        subscriptions += (subs.value.data ?? []).filter((s) => s.status === "active").length;
      }
      if (studs.status === "fulfilled") {
        sawStudents = true;
        students += studs.value.data?.length ?? 0;
      }
      if (staffRes.status === "fulfilled") {
        sawStaff = true;
        staff += staffRes.value.data?.length ?? 0;
      }
    }),
  );

  if (sawSubscriptions) stats.activeSubscriptions = subscriptions;
  if (sawStudents) stats.students = students;
  if (sawStaff) stats.staff = staff;
  return stats;
}

export default async function SuperAdminDashboard() {
  await requireAuth();
  const stats = await loadPlatformStats();

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.03]">
        Console
      </Watermark>
      <Reveal>
        <section className="card card-hover rounded-[var(--radius-md)] p-6">
          <h2 className="font-heading text-xl font-extrabold tracking-tight">
            Welcome to the platform console
          </h2>
          <p className="mt-1 text-sm text-[var(--muted-foreground)]">
            Manage tenants, billing plans, subscriptions, and review platform activity.
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            <Button asChild>
              <Link href="/superadmin/tenants">View tenants</Link>
            </Button>
            <Button asChild variant="secondary">
              <Link href="/superadmin/audit-logs">View audit logs</Link>
            </Button>
          </div>
        </section>
      </Reveal>

      <Reveal delay={80}>
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            label="Tenants"
            value={stats.activeTenants ?? "—"}
            unit="active"
          />
          <StatCard
            label="Subscriptions"
            value={stats.activeSubscriptions ?? "—"}
            unit="active"
            tone="ok"
          />
          <StatCard
            label="Students"
            value={stats.students ?? "—"}
            unit="enrolled"
          />
          <StatCard
            label="Staff"
            value={stats.staff ?? "—"}
            unit="members"
          />
        </section>
      </Reveal>

      <section className="grid gap-6 md:grid-cols-2">
        <Reveal delay={120}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Quick links</h3>
            <ul className="mt-4 space-y-2">
              <QuickLink
                href="/superadmin/tenants"
                icon={<Building2 className="size-4" />}
                label="Tenants"
              />
              <QuickLink
                href="/superadmin/billing-plans"
                icon={<CreditCard className="size-4" />}
                label="Billing plans"
              />
              <QuickLink
                href="/superadmin/subscriptions"
                icon={<Receipt className="size-4" />}
                label="Subscriptions"
              />
              <QuickLink
                href="/superadmin/audit-logs"
                icon={<ScrollText className="size-4" />}
                label="Audit logs"
              />
              <QuickLink
                href="/superadmin/system-health"
                icon={<HeartPulse className="size-4" />}
                label="System health"
              />
            </ul>
          </div>
        </Reveal>
        <Reveal delay={160}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Platform notices</h3>
            <p className="mt-4 text-sm text-[var(--muted-foreground)]">
              Notices and alerts will appear here once the platform health feed is wired.
            </p>
          </div>
        </Reveal>
      </section>
    </div>
  );
}

function QuickLink({ href, icon, label }: { href: string; icon: React.ReactNode; label: string }) {
  return (
    <li>
      <Link
        href={href}
        className="flex items-center gap-3 rounded-[var(--radius-sm)] p-2 text-sm text-[var(--foreground)] transition-colors hover:bg-[var(--muted)]"
      >
        <span className="text-[var(--primary)]">{icon}</span>
        {label}
      </Link>
    </li>
  );
}
