import { CalendarCheck, ClipboardList, FileText } from "lucide-react";
import { DataTable, EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type ReportCard = OpenAPI.report_v1.components["schemas"]["ReportCard"];
type AttendanceRecord = OpenAPI.attendance_v1.components["schemas"]["AttendanceRecord"];

export default async function TeacherReportsPage() {
  const client = await createServerClient();
  const [cardsResult, studentsResult, termsResult, attendanceResult] = await Promise.allSettled([
    client.get<OpenAPI.report_v1.components["schemas"]["ReportCardList"]>(
      "/api/v1/report-cards?limit=100",
    ),
    client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
      "/api/v1/students?limit=100",
    ),
    client.get<OpenAPI.academic_v1.components["schemas"]["TermList"]>("/api/v1/terms?limit=100"),
    client.get<OpenAPI.attendance_v1.components["schemas"]["AttendanceRecordList"]>(
      "/api/v1/attendance?limit=100",
    ),
  ]);

  const cards: ReportCard[] = cardsResult.status === "fulfilled" ? cardsResult.value.data : [];
  const attendance: AttendanceRecord[] =
    attendanceResult.status === "fulfilled" ? (attendanceResult.value.data ?? []) : [];
  const studentNames = new Map(
    (studentsResult.status === "fulfilled" ? (studentsResult.value.data ?? []) : []).map(
      (student) => [student.id, `${student.first_name} ${student.last_name}`],
    ),
  );
  const termNames = new Map(
    (termsResult.status === "fulfilled" ? (termsResult.value.data ?? []) : []).map((term) => [
      term.id,
      term.name,
    ]),
  );
  const reportStatus = (status: ReportCard["status"]) =>
    cards.filter((card) => card.status === status).length;
  const present = attendance.filter(
    (record) => record.status === "present" || record.status === "late",
  ).length;
  const attendanceRate =
    attendance.length > 0 ? `${Math.round((present / attendance.length) * 100)}%` : "—";

  return (
    <div className="space-y-8">
      <PageHeader
        icon={<FileText className="size-6" />}
        title="Reports"
        description="Review report-card progress and recent attendance evidence for your assigned learners."
      />

      <Reveal>
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            label="Draft cards"
            value={cardsResult.status === "fulfilled" ? reportStatus("draft") : "—"}
            unit="in progress"
            tone="warn"
          />
          <StatCard
            label="Generating"
            value={cardsResult.status === "fulfilled" ? reportStatus("generating") : "—"}
            unit="PDF jobs"
          />
          <StatCard
            label="Published"
            value={cardsResult.status === "fulfilled" ? reportStatus("published") : "—"}
            unit="ready"
            tone="ok"
          />
          <StatCard
            label="Attendance"
            value={attendanceResult.status === "fulfilled" ? attendanceRate : "—"}
            unit="latest loaded records"
          />
        </section>
      </Reveal>

      <Reveal delay={80}>
        {cardsResult.status === "rejected" ? (
          <EmptyState
            icon={<FileText className="size-8" />}
            title="Report cards unavailable"
            description="The assigned-learner report feed could not be loaded. No empty report list has been assumed."
          />
        ) : (
          <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="font-sans font-semibold tracking-tight">
                  Assigned learner report cards
                </h2>
                <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                  Draft, generating and published cards returned by your authoritative learner
                  scope.
                </p>
              </div>
              <ClipboardList className="size-5 text-[var(--primary)]" aria-hidden="true" />
            </div>
            <div className="mt-5">
              <DataTable
                caption="Assigned learner report cards"
                rows={cards}
                keyExtractor={(card) => card.id}
                columns={[
                  {
                    key: "student",
                    header: "Learner",
                    cell: (card) => (
                      <span className="font-semibold">
                        {studentNames.get(card.student_id) ?? "Assigned learner"}
                      </span>
                    ),
                  },
                  {
                    key: "term",
                    header: "Term",
                    cell: (card) =>
                      card.term_id
                        ? (termNames.get(card.term_id) ?? "Academic term")
                        : "Not assigned",
                  },
                  {
                    key: "status",
                    header: "Status",
                    cell: (card) => <span className="capitalize">{card.status}</span>,
                  },
                  {
                    key: "updated",
                    header: "Updated",
                    cell: (card) => new Date(card.updated_at).toLocaleDateString("en-GB"),
                  },
                  {
                    key: "download",
                    header: "",
                    cell: (card) =>
                      card.status === "published" ? (
                        <a
                          href={`/api/reports/${card.id}/download`}
                          className="text-sm font-bold text-[var(--primary)] hover:underline"
                        >
                          Download PDF
                        </a>
                      ) : null,
                  },
                ]}
                empty={
                  <EmptyState
                    icon={<FileText className="size-8" />}
                    title="No assigned report cards"
                    description="Report cards for learners in your assigned classes will appear here as they are created."
                  />
                }
              />
            </div>
          </section>
        )}
      </Reveal>

      <Reveal delay={120}>
        {attendanceResult.status === "rejected" ? (
          <EmptyState
            icon={<CalendarCheck className="size-8" />}
            title="Attendance summary unavailable"
            description="Recent assigned-class attendance records could not be loaded."
          />
        ) : (
          <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="font-sans font-semibold tracking-tight">
                  Recent attendance evidence
                </h2>
                <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                  The latest scoped records that contribute to learner reporting.
                </p>
              </div>
              <CalendarCheck className="size-5 text-[var(--primary)]" aria-hidden="true" />
            </div>
            <div className="mt-5">
              <DataTable
                caption="Recent assigned-class attendance"
                rows={attendance.slice(0, 20)}
                keyExtractor={(record) => record.id}
                columns={[
                  {
                    key: "student",
                    header: "Learner",
                    cell: (record) => (
                      <span className="font-semibold">
                        {studentNames.get(record.student_id) ?? "Assigned learner"}
                      </span>
                    ),
                  },
                  {
                    key: "date",
                    header: "Date",
                    cell: (record) =>
                      new Date(`${record.date}T00:00:00`).toLocaleDateString("en-GB"),
                  },
                  {
                    key: "status",
                    header: "Status",
                    cell: (record) => <span className="capitalize">{record.status}</span>,
                  },
                  {
                    key: "reason",
                    header: "Reason",
                    cell: (record) =>
                      record.reason?.trim() ? record.reason.trim() : "No reason recorded",
                  },
                ]}
                empty={
                  <EmptyState
                    icon={<CalendarCheck className="size-8" />}
                    title="No attendance evidence"
                    description="Recent attendance marks for assigned learners will appear here."
                  />
                }
              />
            </div>
          </section>
        )}
      </Reveal>
    </div>
  );
}
