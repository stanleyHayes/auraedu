import Link from "next/link";
import { headers } from "next/headers";
import {
  ArrowRight,
  CalendarDays,
  ClipboardList,
  GraduationCap,
  History,
  Users,
} from "lucide-react";
import { Reveal, StatCard, Watermark } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { getSession, requireAuth } from "@/lib/auth";
import { fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";
import { enabledFeatureKeys, getRouteFeature, isNavigationFeatureVisible } from "@/lib/features";

type StudentList = OpenAPI.student_v1.components["schemas"]["StudentList"];
type StaffList = OpenAPI.staff_v1.components["schemas"]["StaffList"];
type AttendanceList = OpenAPI.attendance_v1.components["schemas"]["AttendanceRecordList"];
type InvoiceList = OpenAPI.fees_v1.components["schemas"]["InvoiceList"];
type AuditList = OpenAPI.audit_v1.components["schemas"]["AuditLogList"];

function boundedCount(data: unknown[] | undefined, nextCursor: string | null | undefined) {
  const count = data?.length ?? 0;
  return nextCursor ? `${count}+` : count;
}

function eventLabel(eventType: string) {
  return eventType
    .replace(/\.v\d+$/, "")
    .replaceAll(".", " ")
    .replaceAll("_", " ");
}

export default async function AdminOverview() {
  await requireAuth();
  const requestHeaders = await headers();
  const [session, client, tenant] = await Promise.all([
    getSession(),
    createServerClient(),
    fetchTenantBranding(getTenantCodeFromHeaders(requestHeaders)),
  ]);
  const enabled = enabledFeatureKeys(tenant.features);
  const quickLinks = [
    {
      href: "/admin/students",
      icon: <Users className="size-4" />,
      label: "Student records",
      detail: "Identity and enrolment",
    },
    {
      href: "/admin/staff",
      icon: <GraduationCap className="size-4" />,
      label: "Staff directory",
      detail: "People and assignments",
    },
    {
      href: "/admin/academic-years",
      icon: <CalendarDays className="size-4" />,
      label: "Academic calendar",
      detail: "Years and terms",
    },
    {
      href: "/admin/admissions",
      icon: <ClipboardList className="size-4" />,
      label: "Admissions review",
      detail: "Applications and decisions",
    },
    // Feature-gated destinations stay hidden while the feature is disabled
    // (agent_plan §2 rule 6), so the dashboard never links into a gated page.
  ].filter((link) => isNavigationFeatureVisible(getRouteFeature(link.href) ?? undefined, enabled));
  const today = new Date().toISOString().slice(0, 10);
  const [students, staff, attendance, pending, partial, overdue, audit] = await Promise.allSettled([
    client.get<StudentList>("/api/v1/students?limit=100"),
    client.get<StaffList>("/api/v1/staff?limit=100"),
    client.get<AttendanceList>(`/api/v1/attendance?date=${today}&limit=100`),
    client.get<InvoiceList>("/api/v1/invoices?status=pending&limit=100"),
    client.get<InvoiceList>("/api/v1/invoices?status=partial&limit=100"),
    client.get<InvoiceList>("/api/v1/invoices?status=overdue&limit=100"),
    client.get<AuditList>("/api/v1/audit/logs?limit=5"),
  ]);

  const studentCount =
    students.status === "fulfilled"
      ? boundedCount(students.value.data, students.value.next_cursor)
      : null;
  const staffCount =
    staff.status === "fulfilled" ? boundedCount(staff.value.data, staff.value.next_cursor) : null;
  const attendanceCount =
    attendance.status === "fulfilled"
      ? boundedCount(attendance.value.data, attendance.value.next_cursor)
      : null;
  const invoiceLists = [pending, partial, overdue];
  const invoicesAvailable = invoiceLists.every((result) => result.status === "fulfilled");
  const openInvoiceCount = invoicesAvailable
    ? invoiceLists.reduce(
        (total, result) => total + (result.status === "fulfilled" ? result.value.data.length : 0),
        0,
      )
    : null;
  const openInvoiceHasMore =
    invoicesAvailable &&
    invoiceLists.some(
      (result) => result.status === "fulfilled" && Boolean(result.value.next_cursor),
    );
  const activity = audit.status === "fulfilled" ? (audit.value.data ?? []) : null;
  const displayName = session?.name ?? session?.email;

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.03]">
        School
      </Watermark>
      <Reveal>
        <section className="portal-hero card card-hover p-6 sm:p-8">
          <div className="flex flex-col justify-between gap-5 sm:flex-row sm:items-end">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.16em] text-[var(--primary)]">
                School command centre
              </p>
              <h2 className="mt-2 font-heading text-2xl font-extrabold tracking-tight">
                {displayName ? `Welcome back, ${displayName}` : "Welcome back"}
              </h2>
              <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                A live view of the people, records and activity shaping today.
              </p>
            </div>
            {isNavigationFeatureVisible("student_management", enabled) && (
              <Link
                href="/admin/students"
                className="inline-flex min-h-11 items-center justify-center gap-2 rounded-[var(--radius-sm)] bg-[var(--primary)] px-4 text-sm font-bold text-[var(--primary-foreground)] transition-transform hover:-translate-y-0.5"
              >
                Open student records <ArrowRight className="size-4" aria-hidden="true" />
              </Link>
            )}
          </div>
        </section>
      </Reveal>

      <Reveal delay={80}>
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Students" value={studentCount ?? "—"} unit="records visible" />
          <StatCard label="Staff" value={staffCount ?? "—"} unit="records visible" />
          <StatCard
            label="Attendance"
            value={attendanceCount ?? "—"}
            unit="marked today"
            tone="ok"
          />
          <StatCard
            label="Open invoices"
            value={
              openInvoiceCount === null
                ? "—"
                : `${openInvoiceCount}${openInvoiceHasMore ? "+" : ""}`
            }
            unit="requiring attention"
            tone="warn"
          />
        </section>
      </Reveal>

      <section className="grid gap-6 lg:grid-cols-[0.8fr_1.2fr]">
        <Reveal delay={120}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Move into the work</h3>
            <p className="mt-1 text-sm text-[var(--muted-foreground)]">
              The most common school administration paths.
            </p>
            <ul className="mt-4 space-y-2">
              {quickLinks.map((link) => (
                <QuickLink
                  key={link.href}
                  href={link.href}
                  icon={link.icon}
                  label={link.label}
                  detail={link.detail}
                />
              ))}
            </ul>
          </div>
        </Reveal>
        <Reveal delay={160}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h3 className="font-sans font-semibold tracking-tight">Recent activity</h3>
                <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                  Latest audited changes in this school.
                </p>
              </div>
              <History className="size-5 text-[var(--primary)]" aria-hidden="true" />
            </div>
            {activity === null ? (
              <div className="mt-5 rounded-[var(--radius-sm)] border border-dashed border-[var(--border)] p-4 text-sm text-[var(--muted-foreground)]">
                The audit feed is temporarily unavailable.
              </div>
            ) : activity.length > 0 ? (
              <ol className="mt-4 divide-y divide-[var(--border)]">
                {activity.map((entry) => (
                  <li
                    key={entry.id}
                    className="grid grid-cols-[auto_1fr] gap-3 py-3 first:pt-0 last:pb-0"
                  >
                    <span
                      className="mt-1 size-2 rounded-full bg-[var(--primary)]"
                      aria-hidden="true"
                    />
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold capitalize">
                        {eventLabel(entry.event_type)}
                      </p>
                      <p className="mt-1 text-xs text-[var(--muted-foreground)]">
                        {entry.resource_type ? `${entry.resource_type} · ` : ""}
                        {new Date(entry.occurred_at).toLocaleString("en-GB")}
                      </p>
                    </div>
                  </li>
                ))}
              </ol>
            ) : (
              <div className="mt-5 rounded-[var(--radius-sm)] border border-dashed border-[var(--border)] p-4 text-sm text-[var(--muted-foreground)]">
                No audited activity has been recorded yet.
              </div>
            )}
          </div>
        </Reveal>
      </section>
    </div>
  );
}

function QuickLink({
  href,
  icon,
  label,
  detail,
}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  detail: string;
}) {
  return (
    <li>
      <Link
        href={href}
        className="group flex items-center gap-3 rounded-[var(--radius-sm)] border border-transparent p-2.5 transition-colors hover:border-[var(--border)] hover:bg-[var(--muted)]"
      >
        <span className="grid size-9 shrink-0 place-items-center rounded-[var(--radius-sm)] bg-[var(--color-brand-tint)] text-[var(--primary)]">
          {icon}
        </span>
        <span className="min-w-0 flex-1">
          <strong className="block text-sm">{label}</strong>
          <small className="text-[var(--muted-foreground)]">{detail}</small>
        </span>
        <ArrowRight
          className="size-4 text-[var(--muted-foreground)] transition-transform group-hover:translate-x-1"
          aria-hidden="true"
        />
      </Link>
    </li>
  );
}
