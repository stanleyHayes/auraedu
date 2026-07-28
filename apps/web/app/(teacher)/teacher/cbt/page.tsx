import Link from "next/link";
import { MonitorCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import {
  examStatusStyle,
  formatExamWindow,
  type Exam,
  type ExamList,
  type SubmissionList,
  type SubmissionRow,
} from "@/lib/teacher-cbt-utils";
import { TeacherCbtExamFormSheet } from "@/components/teacher-cbt-exam-form-sheet";
import { TeacherCbtExamStatusButton } from "@/components/teacher-cbt-exam-status-button";
import { TeacherCbtDeleteExamButton } from "@/components/teacher-cbt-delete-exam-button";

type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

export default async function TeacherCbtPage() {
  let exams: Exam[] = [];
  let subjects: Subject[] = [];
  let years: AcademicYear[] = [];
  let submissions: SubmissionRow[] = [];
  let error: string | null = null;
  let submissionCountAvailable = true;

  try {
    const client = await createServerClient();
    const [examResult, subjectResult, yearResult, submissionResult] = await Promise.allSettled([
      client.get<ExamList>("/api/v1/cbt/exams?limit=50"),
      client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
        "/api/v1/subjects?limit=100",
      ),
      client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
        "/api/v1/academic-years?limit=50",
      ),
      client.get<SubmissionList>("/api/v1/cbt/submissions?limit=100"),
    ]);
    if (examResult.status === "fulfilled") {
      exams = examResult.value.data ?? [];
    } else {
      error =
        examResult.reason instanceof Error ? examResult.reason.message : "Failed to load exams";
    }
    subjects = subjectResult.status === "fulfilled" ? (subjectResult.value.data ?? []) : [];
    years = yearResult.status === "fulfilled" ? (yearResult.value.data ?? []) : [];
    if (submissionResult.status === "fulfilled") {
      submissions = submissionResult.value.data ?? [];
    } else {
      submissionCountAvailable = false;
    }
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load exams";
  }

  const subjectName = new Map(subjects.map((subject) => [subject.id, subject.name]));
  const yearName = new Map(years.map((year) => [year.id, year.name]));
  // The CBT backend is adding an exam reference to submissions; count per exam
  // only when rows carry it, otherwise point teachers at the exam page.
  const submissionsCorrelate = submissions.some((submission) => submission.exam_id);
  const countsByExam = new Map<string, number>();
  for (const submission of submissions) {
    if (!submission.exam_id) continue;
    countsByExam.set(submission.exam_id, (countsByExam.get(submission.exam_id) ?? 0) + 1);
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<MonitorCheck className="size-6" />}
        title="CBT exams"
        description="Author computer-based exams for your assigned subjects, then publish and activate them for students."
        action={<TeacherCbtExamFormSheet mode="create" subjects={subjects} years={years} />}
      />

      {error ? (
        <EmptyState
          title="Could not load exams"
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
                  href={`/teacher/cbt/${exam.id}`}
                >
                  {exam.title}
                </Link>
              ),
            },
            {
              key: "subject",
              header: "Subject / year",
              cell: (exam) => (
                <div>
                  <p className="text-sm">
                    {subjectName.get(exam.subject_id) ?? "Subject unavailable"}
                  </p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {exam.academic_year_id
                      ? (yearName.get(exam.academic_year_id) ?? "Year unavailable")
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
              key: "questions",
              header: "Questions",
              cell: (exam) => (
                <span className="font-heading text-lg font-black">
                  {exam.question_ids?.length ?? 0}
                </span>
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
            {
              key: "actions",
              header: "Actions",
              className: "w-28",
              cell: (exam) => (
                <div className="flex items-center gap-2">
                  <TeacherCbtExamFormSheet
                    mode="edit"
                    initial={exam}
                    subjects={subjects}
                    years={years}
                  />
                  <TeacherCbtExamStatusButton exam={exam} />
                  {!exam.status || exam.status === "draft" ? (
                    <TeacherCbtDeleteExamButton id={exam.id} title={exam.title} />
                  ) : null}
                </div>
              ),
            },
          ]}
          empty={
            <EmptyState
              title="No exams yet"
              description="Create your first computer-based exam to start authoring questions for your classes."
              icon={<MonitorCheck className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
