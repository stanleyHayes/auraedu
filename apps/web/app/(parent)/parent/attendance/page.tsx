import { CalendarCheck, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type AttendanceRecord = OpenAPI.attendance_v1.components["schemas"]["AttendanceRecord"];

export default async function ParentAttendancePage() {
  const client = await createServerClient();
  let records: AttendanceRecord[] = [];
  let students: Record<string, string> = {};
  let error: string | null = null;
  try {
    const [list, family] = await Promise.all([
      client.get<OpenAPI.attendance_v1.components["schemas"]["AttendanceRecordList"]>(
        "/api/v1/attendance?limit=100",
      ),
      client.get<OpenAPI.student_v1.components["schemas"]["GuardianChildren"]>(
        "/api/v1/guardians/me/children",
      ),
    ]);
    records = list.data ?? [];
    students = Object.fromEntries(
      family.students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
    );
  } catch {
    error = "Attendance records for your linked children could not be loaded right now.";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarCheck className="size-6" />}
        title="Attendance"
        description="Attendance records for your children."
      />
      {error ? (
        <EmptyState
          icon={<ClipboardList className="size-8" />}
          title="Attendance unavailable"
          description={error}
        />
      ) : (
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="font-sans font-semibold tracking-tight">Attendance records</h3>
          <div className="mt-4">
            <DataTable
              caption="Attendance records for your children"
              columns={[
                {
                  key: "student_id",
                  header: "Learner",
                  cell: (r) => (
                    <span className="font-semibold">
                      {students[r.student_id] ?? "Linked learner"}
                    </span>
                  ),
                },
                {
                  key: "date",
                  header: "Date",
                  cell: (r) =>
                    new Date(`${r.date}T00:00:00Z`).toLocaleDateString("en-GH", {
                      dateStyle: "medium",
                      timeZone: "UTC",
                    }),
                },
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
      )}
    </div>
  );
}
