"use client";

import { Users, GraduationCap, CalendarDays, ClipboardList } from "lucide-react";
import { StatCard, Button } from "@auraedu/ui";

export default function AdminOverview() {
  return (
    <div className="space-y-8">
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-6">
        <h2 className="font-heading text-xl font-extrabold tracking-tight">Welcome back</h2>
        <p className="mt-1 text-sm text-[var(--muted-foreground)]">
          Here is what is happening at your school today.
        </p>
        <div className="mt-4 flex flex-wrap gap-3">
          <Button onClick={() => (window.location.href = "/admin/students")}>View students</Button>
          <Button variant="secondary" onClick={() => (window.location.href = "/admin/staff")}>
            View staff
          </Button>
        </div>
      </section>

      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Students" value="—" unit="enrolled" />
        <StatCard label="Staff" value="—" unit="members" />
        <StatCard label="Attendance" value="—" unit="today" tone="ok" />
        <StatCard label="Fee arrears" value="—" unit="outstanding" tone="warn" />
      </section>

      <section className="grid gap-6 md:grid-cols-2">
        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="font-sans font-semibold tracking-tight">Quick links</h3>
          <ul className="mt-4 space-y-2">
            <QuickLink
              href="/admin/students"
              icon={<Users className="size-4" />}
              label="Students"
            />
            <QuickLink
              href="/admin/staff"
              icon={<GraduationCap className="size-4" />}
              label="Staff"
            />
            <QuickLink
              href="/admin/academic-years"
              icon={<CalendarDays className="size-4" />}
              label="Academic years"
            />
            <QuickLink
              href="/admin/assessments"
              icon={<ClipboardList className="size-4" />}
              label="Assessments"
            />
          </ul>
        </div>

        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="font-sans font-semibold tracking-tight">Recent activity</h3>
          <p className="mt-4 text-sm text-[var(--muted-foreground)]">
            Recent actions will appear here once the audit feed is wired.
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
