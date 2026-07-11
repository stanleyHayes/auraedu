"use client";

import {
  Building2,
  CreditCard,
  Receipt,
  ScrollText,
  HeartPulse,
} from "lucide-react";
import { StatCard, Button } from "@auraedu/ui";

export default function SuperAdminDashboard() {
  return (
    <div className="space-y-8">
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-6">
        <h2 className="font-display text-xl font-extrabold tracking-tight">
          Welcome to the platform console
        </h2>
        <p className="mt-1 text-sm text-[var(--muted-foreground)]">
          Manage tenants, billing plans, subscriptions, and review platform activity.
        </p>
        <div className="mt-4 flex flex-wrap gap-3">
          <Button onClick={() => (window.location.href = "/superadmin/tenants")}>
            View tenants
          </Button>
          <Button
            variant="secondary"
            onClick={() => (window.location.href = "/superadmin/audit-logs")}
          >
            View audit logs
          </Button>
        </div>
      </section>

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Tenants" value="—" unit="active" />
        <StatCard label="Subscriptions" value="—" unit="paid" tone="ok" />
        <StatCard label="Audit events" value="—" unit="24h" />
        <StatCard label="System status" value="—" unit="healthy" tone="ok" />
      </section>

      <section className="grid gap-6 md:grid-cols-2">
        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="font-display font-semibold tracking-tight">Quick links</h3>
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

        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="font-display font-semibold tracking-tight">Platform notices</h3>
          <p className="mt-4 text-sm text-[var(--muted-foreground)]">
            Notices and alerts will appear here once the platform health feed is wired.
          </p>
        </div>
      </section>
    </div>
  );
}

function QuickLink({ href, icon, label }: { href: string; icon: React.ReactNode; label: string }) {
  return (
    <li>
      <a
        href={href}
        className="flex items-center gap-3 rounded-[var(--radius-sm)] p-2 text-sm text-[var(--foreground)] transition-colors hover:bg-[var(--muted)]"
      >
        <span className="text-[var(--primary)]">{icon}</span>
        {label}
      </a>
    </li>
  );
}
