import { CalendarCheck2 } from "lucide-react";
import { PageHeader, DataTable, EmptyState, StatCard, Reveal } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClientForTenant } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { drilldownQuery, nextPageHref } from "@/lib/superadmin-drilldown";
import { SuperadminPagination } from "@/components/superadmin-pagination";

type AttendanceRecord = OpenAPI.attendance_v1.components["schemas"]["AttendanceRecord"];

interface TenantAttendancePageProps {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ cursor?: string }>;
}

export default async function TenantAttendancePage({
  params,
  searchParams,
}: TenantAttendancePageProps) {
  const { code } = await params;
  const { cursor } = await searchParams;
  await requireAuth();

  const today = new Date().toISOString().slice(0, 10);

  let records: AttendanceRecord[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClientForTenant(code);
    const res = await client.get<{ data?: AttendanceRecord[]; next_cursor?: string | null }>(
      `/api/v1/attendance?${drilldownQuery({ date: today }, cursor)}`,
    );
    records = res.data ?? [];
    nextCursor = res.next_cursor ?? null;
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load attendance";
  }

  const countByStatus = (status: AttendanceRecord["status"]) =>
    records.filter((r) => r.status === status).length;
  const nextHref = nextPageHref(`/superadmin/tenants/${code}/attendance`, {}, nextCursor);

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          icon={<CalendarCheck2 className="size-7" />}
          title="Attendance"
          description={`Read-only attendance marked today (${today}) for ${code}.`}
        />
      </Reveal>

      {error ? (
        <EmptyState
          title="Attendance unavailable"
          description={error}
          icon={<CalendarCheck2 className="size-8" />}
        />
      ) : (
        <>
          <Reveal delay={60}>
            <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard label="Present" value={countByStatus("present")} unit="today" tone="ok" />
              <StatCard label="Absent" value={countByStatus("absent")} unit="today" />
              <StatCard label="Late" value={countByStatus("late")} unit="today" />
              <StatCard label="Excused" value={countByStatus("excused")} unit="today" />
            </section>
          </Reveal>

          <Reveal delay={100}>
            <DataTable
              caption={`Attendance for tenant ${code} on ${today}`}
              rows={records}
              keyExtractor={(r) => r.id}
              columns={[
                {
                  key: "student",
                  header: "Student",
                  cell: (r) => (
                    <span className="font-mono text-xs" title={r.student_id}>
                      {r.student_id.slice(0, 8)}…
                    </span>
                  ),
                },
                {
                  key: "class",
                  header: "Class",
                  cell: (r) =>
                    r.class_id ? (
                      <span className="font-mono text-xs" title={r.class_id}>
                        {r.class_id.slice(0, 8)}…
                      </span>
                    ) : (
                      "—"
                    ),
                },
                {
                  key: "status",
                  header: "Status",
                  className: "w-28",
                  cell: (r) =>
                    r.status === "present" ? (
                      <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                        {r.status}
                      </span>
                    ) : r.status === "absent" ? (
                      <span className="rounded-full bg-[var(--color-warn)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-warn)]">
                        {r.status}
                      </span>
                    ) : (
                      <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                        {r.status}
                      </span>
                    ),
                },
                {
                  key: "reason",
                  header: "Reason",
                  cell: (r) => r.reason ?? "—",
                },
                {
                  key: "marked_by",
                  header: "Marked by",
                  cell: (r) => <span className="font-mono text-xs">{r.marked_by}</span>,
                },
              ]}
              empty={
                <EmptyState
                  title="No attendance marked today"
                  description="Records will appear here once teachers mark the register."
                  icon={<CalendarCheck2 className="size-8" />}
                />
              }
            />
          </Reveal>

          <SuperadminPagination nextHref={nextHref} />
        </>
      )}
    </div>
  );
}
