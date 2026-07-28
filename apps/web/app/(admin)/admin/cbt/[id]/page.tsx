import Link from "next/link";
import { ArrowLeft, MonitorCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import {
  examStatusStyle,
  formatExamWindow,
  submissionStatusStyle,
  type Exam,
  type SubmissionRow,
} from "@/lib/admin-cbt";

type SubmissionList = OpenAPI.cbt_v1.components["schemas"]["SubmissionList"];
type Student = OpenAPI.student_v1.components["schemas"]["Student"];

function formatTime(value: string | null | undefined) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("en-GH", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value),
  );
}

export default async function AdminCbtExamPage({ params }: { params: Promise<{ id: string }> }) {
  await requireAuth();
  const { id } = await params;

  let exam: Exam | null = null;
  let submissions: SubmissionRow[] = [];
  let students: Student[] = [];
  let error: string | null = null;
  let submissionsError: string | null = null;

  try {
    const client = await createServerClient();
    const [examResult, submissionResult, studentResult] = await Promise.allSettled([
      client.get<Exam>(`/api/v1/cbt/exams/${encodeURIComponent(id)}`),
      client.get<SubmissionList>(
        `/api/v1/cbt/submissions?limit=100&exam_id=${encodeURIComponent(id)}`,
      ),
      client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
        "/api/v1/students?limit=100",
      ),
    ]);
    if (examResult.status === "fulfilled") {
      exam = examResult.value;
    } else {
      error =
        examResult.reason instanceof Error ? examResult.reason.message : "Failed to load the exam";
    }
    if (submissionResult.status === "fulfilled") {
      submissions = submissionResult.value.data ?? [];
    } else {
      submissionsError =
        submissionResult.reason instanceof Error
          ? submissionResult.reason.message
          : "Failed to load submissions";
    }
    students = studentResult.status === "fulfilled" ? (studentResult.value.data ?? []) : [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load the exam";
  }

  // The CBT backend is adding exam scoping to submissions; until rows carry an
  // exam reference, trust the query filter and say so rather than misattribute.
  const correlated = submissions.some((submission) => submission.exam_id);
  const visibleSubmissions = correlated
    ? submissions.filter((submission) => submission.exam_id === id)
    : submissions;
  const studentNames = new Map(
    students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
  );
  const graded = visibleSubmissions.filter((submission) => submission.status === "graded").length;

  if (error || !exam) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<MonitorCheck className="size-7" />}
          title="Exam detail"
          description="The exam could not be loaded."
        />
        <EmptyState
          title="Could not load the exam"
          description={error ?? "The exam does not exist or is not visible to your account."}
          icon={<MonitorCheck className="size-8" />}
        />
        <Link
          href="/admin/cbt"
          className="inline-flex items-center gap-2 text-sm font-semibold text-primary underline-offset-4 hover:underline"
        >
          <ArrowLeft className="size-4" /> Back to CBT exams
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <Link
        href="/admin/cbt"
        className="inline-flex items-center gap-2 text-sm font-semibold text-muted-foreground underline-offset-4 transition hover:text-primary hover:underline"
      >
        <ArrowLeft className="size-4" /> All CBT exams
      </Link>
      <PageHeader
        icon={<MonitorCheck className="size-7" />}
        title={exam.title}
        description={`${formatExamWindow(exam)} · ${exam.duration_minutes ?? "—"} minutes · ${exam.question_ids?.length ?? 0} questions. Read-only oversight — grading stays with teaching staff.`}
        action={
          <span
            className={`rounded-full px-3 py-1.5 text-xs font-semibold capitalize ${examStatusStyle(exam.status)}`}
          >
            {exam.status ?? "draft"}
          </span>
        }
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Submissions" value={visibleSubmissions.length} unit="attempts" />
        <StatCard label="Graded" value={graded} tone="ok" />
        <StatCard
          label="Awaiting submission"
          value={
            visibleSubmissions.filter((submission) => submission.status === "in_progress").length
          }
        />
      </div>

      {submissionsError ? (
        <EmptyState
          title="Could not load submissions"
          description={submissionsError}
          icon={<MonitorCheck className="size-8" />}
        />
      ) : (
        <>
          {!correlated && visibleSubmissions.length > 0 ? (
            <div className="rounded-xl border border-amber-300/60 bg-amber-50 p-4 text-sm text-amber-950">
              Submission records do not carry an exam reference yet, so this list relies on the exam
              filter being honoured by the CBT service.
            </div>
          ) : null}
          <DataTable
            caption={`Submissions for ${exam.title}`}
            rows={visibleSubmissions}
            keyExtractor={(submission) => submission.id}
            columns={[
              {
                key: "student",
                header: "Student",
                cell: (submission) =>
                  studentNames.get(submission.student_id) ? (
                    <span className="font-semibold">{studentNames.get(submission.student_id)}</span>
                  ) : (
                    <span className="font-mono text-xs">{submission.student_id.slice(0, 8)}…</span>
                  ),
              },
              {
                key: "status",
                header: "Status",
                cell: (submission) => (
                  <span
                    className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${submissionStatusStyle(submission.status)}`}
                  >
                    {submission.status.replaceAll("_", " ")}
                  </span>
                ),
              },
              {
                key: "score",
                header: "Score",
                cell: (submission) =>
                  submission.score == null ? (
                    <span className="text-muted-foreground">Not graded</span>
                  ) : (
                    <span>
                      <span className="font-heading text-lg font-black">{submission.score}</span>
                      {submission.max_score != null ? (
                        <span className="ml-1 text-xs text-muted-foreground">
                          / {submission.max_score}
                        </span>
                      ) : null}
                    </span>
                  ),
              },
              {
                key: "submitted_at",
                header: "Submitted at",
                cell: (submission) => (
                  <span className="whitespace-nowrap text-sm">
                    {formatTime(submission.submitted_at)}
                  </span>
                ),
              },
            ]}
            empty={
              <EmptyState
                title="No submissions yet"
                description="Student attempts will appear here once the exam window opens."
                icon={<MonitorCheck className="size-8" />}
              />
            }
          />
        </>
      )}
    </div>
  );
}
