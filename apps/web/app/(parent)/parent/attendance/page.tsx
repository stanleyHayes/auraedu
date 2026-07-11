import { CalendarCheck, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";

export interface AttendanceRecord {
  id: string;
  student_id: string;
  date: string;
  status: string;
}

export default async function ParentAttendancePage() {
  const client = await createServerClient();
  let records: AttendanceRecord[] = [];
  try {
    records = await client.get<AttendanceRecord[]>("/api/v1/attendance");
  } catch {
    records = [];
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarCheck className="size-6" />}
        title="Attendance"
        description="Attendance records for your children."
      />
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-display font-semibold tracking-tight">Attendance records</h3>
        <div className="mt-4">
          <DataTable
            caption="Attendance records for your children"
            columns={[
              { key: "student_id", header: "Student ID", cell: (r) => r.student_id },
              { key: "date", header: "Date", cell: (r) => r.date },
              {
                key: "status",
                header: "Status",
                cell: (r) => <span className="capitalize">{r.status}</span>,
              },
            ]}
            rows={records}
            keyExtractor={(r) => r.id}
            empty={
              <EmptyState
                icon={<ClipboardList className="size-8" />}
                title="No attendance records"
                description="Records will appear here once attendance is recorded for your children."
              />
            }
          />
        </div>
      </section>
    </div>
  );
}
