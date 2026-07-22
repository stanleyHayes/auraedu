import { CalendarCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

export default async function AdminAttendancePage() {
  const client = await createServerClient();
  try {
    const list = await client.get<
      OpenAPI.attendance_v1.components["schemas"]["AttendanceRecordList"]
    >("/api/v1/attendance?limit=100");
    const rows = list.data ?? [];
    const count = (status: string) => rows.filter((record) => record.status === status).length;
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<CalendarCheck className="size-6" />}
          title="Attendance control"
          description="Review the latest tenant attendance records and exceptions."
        />
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Present" value={count("present")} unit="loaded records" tone="ok" />
          <StatCard label="Late" value={count("late")} unit="loaded records" tone="warn" />
          <StatCard label="Absent" value={count("absent")} unit="loaded records" />
          <StatCard label="Excused" value={count("excused")} unit="loaded records" />
        </section>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <DataTable
            caption="Latest attendance records"
            rows={rows}
            keyExtractor={(record) => record.id}
            columns={[
              {
                key: "student",
                header: "Student ID",
                cell: (record) => <span className="font-mono text-xs">{record.student_id}</span>,
              },
              { key: "date", header: "Date", cell: (record) => record.date },
              {
                key: "status",
                header: "Status",
                cell: (record) => <span className="capitalize">{record.status}</span>,
              },
              {
                key: "reason",
                header: "Reason",
                cell: (record) => (record.reason?.trim() ? record.reason : "No reason recorded"),
              },
            ]}
            empty={
              <EmptyState
                icon={<CalendarCheck className="size-8" />}
                title="No attendance records"
                description="Attendance evidence will appear once registers are marked."
              />
            }
          />
        </section>
      </div>
    );
  } catch {
    return (
      <EmptyState
        icon={<CalendarCheck className="size-8" />}
        title="Attendance unavailable"
        description="The attendance service could not be reached. No empty register has been assumed."
      />
    );
  }
}
