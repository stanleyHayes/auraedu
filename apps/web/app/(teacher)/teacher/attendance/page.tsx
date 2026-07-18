import { CalendarCheck } from "lucide-react";
import { PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { TeacherAttendanceClient } from "@/components/teacher-attendance-client";

export type AttendanceRecord = OpenAPI.attendance_v1.components["schemas"]["AttendanceRecord"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Student = OpenAPI.student_v1.components["schemas"]["Student"];

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

  // Supplemental lookups for the mark-attendance form are best-effort so the
  // records list still renders when a related service is unavailable.
  let classes: AcademicClass[];
  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>(
      "/api/v1/classes?limit=100",
    );
    classes = res.data ?? [];
  } catch {
    classes = [];
  }

  let years: AcademicYear[];
  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
      "/api/v1/academic-years?limit=50",
    );
    years = res.data ?? [];
  } catch {
    years = [];
  }

  // NOTE: GET /api/v1/students has no class_id filter and Student carries no
  // class_id, so a true class roster is not fetchable yet — the client lists
  // all active students and tags submitted records with the selected class.
  let students: Student[];
  try {
    const res = await client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
      "/api/v1/students?limit=100",
    );
    students = res.data ?? [];
  } catch {
    students = [];
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarCheck className="size-6" />}
        title="Attendance"
        description="Review and record attendance for students in your tenant."
      />
      <TeacherAttendanceClient
        initialRecords={records}
        classes={classes}
        years={years}
        students={students}
      />
    </div>
  );
}
