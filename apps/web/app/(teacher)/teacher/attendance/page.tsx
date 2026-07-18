import { CalendarCheck } from "lucide-react";
import { PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { TeacherAttendanceClient } from "@/components/teacher-attendance-client";

export type AttendanceRecord = OpenAPI.attendance_v1.components["schemas"]["AttendanceRecord"];

export default async function TeacherAttendancePage() {
  const client = await createServerClient();
  let records: AttendanceRecord[];
  try {
    const res = await client.get<
      OpenAPI.attendance_v1.components["schemas"]["AttendanceRecordList"]
    >("/api/v1/attendance");
    records = res.data ?? [];
  } catch {
    records = [];
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarCheck className="size-6" />}
        title="Attendance"
        description="Review and record attendance for students in your tenant."
      />
      <TeacherAttendanceClient initialRecords={records} />
    </div>
  );
}
