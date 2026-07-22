import { getSession } from "@/lib/auth";
import { ParentDashboard } from "@/components/parent-dashboard";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

export default async function ParentDashboardPage() {
  const session = await getSession();
  const client = await createServerClient();
  const [family, attendance, invoices, assessments] = await Promise.allSettled([
    client.get<OpenAPI.student_v1.components["schemas"]["GuardianChildren"]>(
      "/api/v1/guardians/me/children",
    ),
    client.get<OpenAPI.attendance_v1.components["schemas"]["AttendanceRecordList"]>(
      "/api/v1/attendance?limit=100",
    ),
    client.get<OpenAPI.fees_v1.components["schemas"]["InvoiceList"]>("/api/v1/invoices"),
    client.get<OpenAPI.assessment_v1.components["schemas"]["AssessmentList"]>(
      "/api/v1/assessments?status=published&limit=50",
    ),
  ]);

  const sevenDaysAgo = new Date();
  sevenDaysAgo.setUTCDate(sevenDaysAgo.getUTCDate() - 6);
  sevenDaysAgo.setUTCHours(0, 0, 0, 0);
  const recentAttendance =
    attendance.status === "fulfilled"
      ? (attendance.value.data ?? []).filter((record) => new Date(record.date) >= sevenDaysAgo)
      : null;
  const attended =
    recentAttendance?.filter((record) => record.status === "present" || record.status === "late")
      .length ?? 0;
  const attendanceRate =
    recentAttendance === null
      ? null
      : recentAttendance.length === 0
        ? "—"
        : `${Math.round((attended / recentAttendance.length) * 100)}%`;

  let publishedResults: number | null = null;
  if (assessments.status === "fulfilled") {
    try {
      const lists = await Promise.all(
        (assessments.value.data ?? []).map((assessment) =>
          client.get<OpenAPI.assessment_v1.components["schemas"]["ScoreList"]>(
            `/api/v1/assessments/${encodeURIComponent(assessment.id)}/scores?limit=100`,
          ),
        ),
      );
      publishedResults = lists.reduce((total, list) => total + (list.data?.length ?? 0), 0);
    } catch {
      publishedResults = null;
    }
  }

  return (
    <ParentDashboard
      userName={session?.name ?? session?.email ?? undefined}
      summary={{
        children:
          family.status === "fulfilled"
            ? family.value.students.map((student) => ({
                id: student.id,
                name: `${student.first_name} ${student.last_name}`,
                code: student.student_code,
                status: student.status ?? "active",
              }))
            : null,
        attendanceRate,
        openInvoices:
          invoices.status === "fulfilled"
            ? (invoices.value.data ?? []).filter(
                (invoice) =>
                  invoice.status === "pending" ||
                  invoice.status === "partial" ||
                  invoice.status === "overdue",
              ).length
            : null,
        publishedResults,
      }}
    />
  );
}
