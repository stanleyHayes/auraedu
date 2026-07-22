import { getSession } from "@/lib/auth";
import { TeacherDashboard } from "@/components/teacher-dashboard";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

export default async function TeacherDashboardPage() {
  const session = await getSession();
  const client = await createServerClient();
  const [classes, timetable, subjects, assignments, announcements] = await Promise.allSettled([
    client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>("/api/v1/classes?limit=100"),
    client.get<OpenAPI.academic_v1.components["schemas"]["TimetableList"]>("/api/v1/timetable"),
    client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=100",
    ),
    client.get<OpenAPI.assessment_v1.components["schemas"]["AssignmentList"]>(
      "/api/v1/assignments?limit=100",
    ),
    client.get<OpenAPI.notification_v1.components["schemas"]["AnnouncementList"]>(
      "/api/v1/announcements?limit=3&audience=staff",
    ),
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
  const assignmentRows = assignments.status === "fulfilled" ? (assignments.value.data ?? []) : null;

  return (
    <TeacherDashboard
      userName={session?.name ?? session?.email ?? undefined}
      summary={{
        assignedClasses: classes.status === "fulfilled" ? (classes.value.data?.length ?? 0) : null,
        classesToday: timetable.status === "fulfilled" ? lessons.length : null,
        activeAssignments:
          assignmentRows === null
            ? null
            : assignmentRows.filter((assignment) => assignment.status === "published").length,
        draftAssignments:
          assignmentRows === null
            ? null
            : assignmentRows.filter((assignment) => assignment.status !== "published").length,
        lessons: timetable.status === "fulfilled" ? lessons : null,
        announcements:
          announcements.status === "fulfilled"
            ? (announcements.value.data ?? []).slice(0, 3)
            : null,
      }}
    />
  );
}
