import { getSession } from "@/lib/auth";
import { StudentDashboard } from "@/components/student-dashboard";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

export default async function StudentDashboardPage() {
  const session = await getSession();
  const client = await createServerClient();
  const [timetable, subjects, assignments, assessments, exams] = await Promise.allSettled([
    client.get<OpenAPI.academic_v1.components["schemas"]["TimetableList"]>("/api/v1/timetable"),
    client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=100",
    ),
    client.get<OpenAPI.assessment_v1.components["schemas"]["AssignmentList"]>(
      "/api/v1/assignments?status=published&limit=50",
    ),
    client.get<OpenAPI.assessment_v1.components["schemas"]["AssessmentList"]>(
      "/api/v1/assessments?status=published&limit=50",
    ),
    client.get<OpenAPI.cbt_v1.components["schemas"]["ExamList"]>("/api/v1/cbt/exams"),
  ]);

  const entries = timetable.status === "fulfilled" ? timetable.value.data : [];
  const subjectMap = new Map(
    (subjects.status === "fulfilled" ? (subjects.value.data ?? []) : []).map((subject) => [
      subject.id,
      subject.name,
    ]),
  );
  const today = new Date().getDay() || 7;
  const lessons = entries
    .filter((entry) => entry.status === "active" && entry.weekday === today)
    .sort((left, right) => left.start_time.localeCompare(right.start_time))
    .map((entry) => ({
      id: entry.id,
      time: `${entry.start_time}–${entry.end_time}`,
      title: subjectMap.get(entry.subject_id) ?? "Scheduled lesson",
      room: entry.room,
    }));

  let publishedResults: number | null = null;
  if (assessments.status === "fulfilled") {
    try {
      const scores = await Promise.all(
        (assessments.value.data ?? []).map((assessment) =>
          client.get<OpenAPI.assessment_v1.components["schemas"]["ScoreList"]>(
            `/api/v1/assessments/${encodeURIComponent(assessment.id)}/scores?limit=100`,
          ),
        ),
      );
      publishedResults = scores.reduce((total, list) => total + (list.data?.length ?? 0), 0);
    } catch {
      publishedResults = null;
    }
  }

  return (
    <StudentDashboard
      userName={session?.name ?? session?.email ?? undefined}
      summary={{
        classesToday: timetable.status === "fulfilled" ? lessons.length : null,
        activeAssignments:
          assignments.status === "fulfilled" ? (assignments.value.data?.length ?? 0) : null,
        publishedResults,
        upcomingExams:
          exams.status === "fulfilled"
            ? (exams.value.data ?? []).filter(
                (exam) => exam.status === "active" || exam.status === "published",
              ).length
            : null,
        lessons: timetable.status === "fulfilled" ? lessons : null,
      }}
    />
  );
}
