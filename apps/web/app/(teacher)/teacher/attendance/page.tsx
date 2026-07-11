import { CalendarCheck } from "lucide-react";
import { PageHeader } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { TeacherAttendanceClient } from "@/components/teacher-attendance-client";

export interface AttendanceRecord {
  id: string;
  student_id: string;
  date: string;
  status: string;
}

export default async function TeacherAttendancePage() {
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
        description="Review and record attendance for students in your tenant."
      />
      <TeacherAttendanceClient initialRecords={records} />
    </div>
  );
}
