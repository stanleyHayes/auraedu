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
  const [recordResult, classResult, yearResult] = await Promise.allSettled([
    client.get<OpenAPI.attendance_v1.components["schemas"]["AttendanceRecordList"]>(
      "/api/v1/attendance?limit=100",
    ),
    client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>("/api/v1/classes?limit=100"),
    client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
      "/api/v1/academic-years?limit=50",
    ),
  ]);
  const records = recordResult.status === "fulfilled" ? (recordResult.value.data ?? []) : [];
  const classes: AcademicClass[] =
    classResult.status === "fulfilled" ? (classResult.value.data ?? []) : [];
  const years: AcademicYear[] =
    yearResult.status === "fulfilled" ? (yearResult.value.data ?? []) : [];
  const firstClassId = classes[0]?.id ?? "";
  let initialStudents: Student[] | null = [];
  if (firstClassId) {
    try {
      const result = await client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
        `/api/v1/students?class_id=${encodeURIComponent(firstClassId)}&limit=100`,
      );
      initialStudents = (result.data ?? []).filter(
        (student) => !student.status || student.status === "active",
      );
    } catch {
      initialStudents = null;
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarCheck className="size-6" />}
        title="Attendance"
        description="Review and record attendance for learners in your assigned classes."
      />
      <TeacherAttendanceClient
        initialRecords={records}
        recordsAvailable={recordResult.status === "fulfilled"}
        classes={classResult.status === "fulfilled" ? classes : null}
        years={yearResult.status === "fulfilled" ? years : null}
        initialClassId={firstClassId}
        initialStudents={initialStudents}
      />
    </div>
  );
}
