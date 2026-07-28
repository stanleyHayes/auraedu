import Link from "next/link";
import { MonitorCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { examStatusStyle, formatExamWindow, type SubmissionRow } from "@/lib/admin-cbt";

type Exam = OpenAPI.cbt_v1.components["schemas"]["Exam"];
type ExamList = OpenAPI.cbt_v1.components["schemas"]["ExamList"];
type SubmissionList = OpenAPI.cbt_v1.components["schemas"]["SubmissionList"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

export default async function AdminCbtPage() {
  await requireAuth();

  let exams: Exam[] = [];
  let submissions: SubmissionRow[] = [];
  let subjects: Subject[] = [];
  let years: AcademicYear[] = [];
  let error: string | null = null;
  let submissionCountAvailable = true;

  try {
    const client = await createServerClient();
    const [examResult, submissionResult, subjectResult, yearResult] = await Promise.allSettled([
      client.get<ExamList>("/api/v1/cbt/exams?limit=50"),
      client.get<SubmissionList>("/api/v1/cbt/submissions?limit=100"),
      client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
        "/api/v1/subjects?limit=100",
      ),
      client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
        "/api/v1/academic-years?limit=100",
      ),
    ]);
    if (examResult.status === "fulfilled") {
      exams = examResult.value.data ?? [];
    } else {
      error =
        examResult.reason instanceof Error ? examResult.reason.message : "Failed to load CBT exams";
    }
    if (submissionResult.status === "fulfilled") {
      submissions = submissionResult.value.data ?? [];
    } else {
      submissionCountAvailable = false;
    }
    subjects = subjectResult.status === "fulfilled" ? (subjectResult.value.data ?? []) : [];
    years = yearResult.status === "fulfilled" ? (yearResult.value.data ?? []) : [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load CBT exams";
  }

  const subjectNames = new Map(subjects.map((subject) => [subject.id, subject.name]));
  const yearNames = new Map(years.map((year) => [year.id, year.name]));
  const submissionsCorrelate = submissions.some((submission) => submission.exam_id);
  const countsByExam = new Map<string, number>();
  for (const submission of submissions) {
    if (!submission.exam_id) continue;
    countsByExam.set(submission.exam_id, (countsByExam.get(submission.exam_id) ?? 0) + 1);
  }

  const active = exams.filter((exam) => exam.status === "active").length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<MonitorCheck className="size-7" />}
        title="CBT exams"
        description="Oversight of computer-based exams and their submission traffic. Authoring and grading stay with teaching staff."
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Exams" value={exams.length} unit="records" />
        <StatCard label="Active now" value={active} tone={active > 0 ? "ok" : "default"} />
        <StatCard
          label="Submissions tracked"
          value={submissionCountAvailable ? submissions.length : "—"}
          unit="recent"
        />
      </div>

      {error ? (
        <EmptyState
          title="Could not load CBT exams"
          description={error}
          icon={<MonitorCheck className="size-8" />}
        />
      ) : (
        <DataTable
          caption="CBT exams"
          rows={exams}
          keyExtractor={(exam) => exam.id}
          columns={[
            {
              key: "title",
              header: "Exam",
              cell: (exam) => (
                <Link
                  className="font-semibold text-primary underline-offset-4 hover:underline"
                  href={`/admin/cbt/${exam.id}`}
                >
                  {exam.title}
                </Link>
              ),
            },
            {
              key: "subject",
              header: "Class / subject",
              cell: (exam) => (
                <div>
                  <p className="text-sm">
                    {subjectNames.get(exam.subject_id) ?? "Subject unavailable"}
                  </p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {exam.academic_year_id
                      ? (yearNames.get(exam.academic_year_id) ?? "Year unavailable")
                      : "No academic year"}
                  </p>
                </div>
              ),
            },
            {
              key: "status",
              header: "Status",
              cell: (exam) => (
                <span
                  className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${examStatusStyle(exam.status)}`}
                >
                  {exam.status ?? "draft"}
                </span>
              ),
            },
            {
              key: "window",
              header: "Window",
              cell: (exam) => (
                <span className="whitespace-nowrap text-sm">{formatExamWindow(exam)}</span>
              ),
            },
            {
              key: "submissions",
              header: "Submissions",
              cell: (exam) => {
                if (!submissionCountAvailable)
                  return <span className="text-muted-foreground">Unavailable</span>;
                if (!submissionsCorrelate)
                  return <span className="text-muted-foreground">See exam</span>;
                return (
                  <span className="font-heading text-lg font-black">
                    {countsByExam.get(exam.id) ?? 0}
                  </span>
                );
              },
            },
          ]}
          empty={
            <EmptyState
              title="No CBT exams yet"
              description="Exams authored by teaching staff will appear here with their windows and submission counts."
              icon={<MonitorCheck className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
