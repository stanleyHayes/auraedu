import Link from "next/link";
import { ArrowLeft, ListChecks, MonitorCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import {
  examStatusStyle,
  formatExamWindow,
  orderExamQuestions,
  questionTypeLabel,
  submissionStatusStyle,
  sumMarks,
  type Exam,
  type Question,
  type QuestionList,
  type SubmissionList,
  type SubmissionRow,
} from "@/lib/teacher-cbt-utils";
import { TeacherCbtExamFormSheet } from "@/components/teacher-cbt-exam-form-sheet";
import { TeacherCbtExamStatusButton } from "@/components/teacher-cbt-exam-status-button";
import { TeacherCbtQuestionFormSheet } from "@/components/teacher-cbt-question-form-sheet";
import { TeacherCbtDeleteQuestionButton } from "@/components/teacher-cbt-delete-question-button";
import { TeacherCbtAttachQuestionButton } from "@/components/teacher-cbt-attach-question-button";
import { TeacherCbtMoveQuestionButton } from "@/components/teacher-cbt-move-question-button";

type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Student = OpenAPI.student_v1.components["schemas"]["Student"];

function formatTime(value: string | null | undefined) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("en-GH", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value),
  );
}

export default async function TeacherCbtExamPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  let exam: Exam | null = null;
  let bank: Question[] = [];
  let submissions: SubmissionRow[] = [];
  let students: Student[] = [];
  let subjects: Subject[] = [];
  let years: AcademicYear[] = [];
  let error: string | null = null;
  let questionsError: string | null = null;
  let submissionsError: string | null = null;

  try {
    const client = await createServerClient();
    const [examResult, questionResult, submissionResult, studentResult, subjectResult, yearResult] =
      await Promise.allSettled([
        client.get<Exam>(`/api/v1/cbt/exams/${encodeURIComponent(id)}`),
        client.get<QuestionList>("/api/v1/cbt/questions?limit=100"),
        client.get<SubmissionList>(
          `/api/v1/cbt/submissions?limit=100&exam_id=${encodeURIComponent(id)}`,
        ),
        client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
          "/api/v1/students?limit=100",
        ),
        client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
          "/api/v1/subjects?limit=100",
        ),
        client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
          "/api/v1/academic-years?limit=50",
        ),
      ]);
    if (examResult.status === "fulfilled") {
      exam = examResult.value;
    } else {
      error =
        examResult.reason instanceof Error ? examResult.reason.message : "Failed to load the exam";
    }
    if (questionResult.status === "fulfilled") {
      bank = questionResult.value.data ?? [];
    } else {
      questionsError =
        questionResult.reason instanceof Error
          ? questionResult.reason.message
          : "Failed to load questions";
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
    subjects = subjectResult.status === "fulfilled" ? (subjectResult.value.data ?? []) : [];
    years = yearResult.status === "fulfilled" ? (yearResult.value.data ?? []) : [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load the exam";
  }

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
          href="/teacher/cbt"
          className="inline-flex items-center gap-2 text-sm font-semibold text-primary underline-offset-4 hover:underline"
        >
          <ArrowLeft className="size-4" /> Back to CBT exams
        </Link>
      </div>
    );
  }

  // The question bank is tenant-wide; only questions matching the exam's
  // subject and academic year are relevant here.
  const relevantBank = bank.filter(
    (question) =>
      question.subject_id === exam.subject_id &&
      (!exam.academic_year_id || question.academic_year_id === exam.academic_year_id),
  );
  const attached = orderExamQuestions(exam, relevantBank);
  const attachedIds = new Set(attached.map((question) => question.id));
  const available = relevantBank.filter((question) => !attachedIds.has(question.id));
  const totalMarks = sumMarks(attached);

  // The CBT backend is adding exam scoping to submissions; until rows carry an
  // exam reference, trust the query filter and say so rather than misattribute.
  const correlated = submissions.some((submission) => submission.exam_id);
  const visibleSubmissions = correlated
    ? submissions.filter((submission) => submission.exam_id === id)
    : submissions;
  const studentNames = new Map(
    students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
  );

  return (
    <div className="space-y-6">
      <Link
        href="/teacher/cbt"
        className="inline-flex items-center gap-2 text-sm font-semibold text-muted-foreground underline-offset-4 transition hover:text-primary hover:underline"
      >
        <ArrowLeft className="size-4" /> All CBT exams
      </Link>
      <PageHeader
        icon={<MonitorCheck className="size-7" />}
        title={exam.title}
        description={`${formatExamWindow(exam)} · ${exam.duration_minutes ?? "—"} minutes. Author questions below, then publish and activate when ready.`}
        action={
          <div className="flex items-center gap-2">
            <span
              className={`rounded-full px-3 py-1.5 text-xs font-semibold capitalize ${examStatusStyle(exam.status)}`}
            >
              {exam.status ?? "draft"}
            </span>
            <TeacherCbtExamFormSheet mode="edit" initial={exam} subjects={subjects} years={years} />
            <TeacherCbtExamStatusButton exam={exam} />
          </div>
        }
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Questions" value={attached.length} unit="in this exam" />
        <StatCard label="Total marks" value={totalMarks} tone={totalMarks > 0 ? "ok" : "default"} />
        <StatCard label="Duration" value={exam.duration_minutes ?? "—"} unit="minutes" />
      </div>

      <section className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="font-sans font-semibold tracking-tight">Questions in this exam</h2>
          {!questionsError ? <TeacherCbtQuestionFormSheet mode="create" exam={exam} /> : null}
        </div>

        {questionsError ? (
          <EmptyState
            title="Could not load questions"
            description={questionsError}
            icon={<ListChecks className="size-8" />}
          />
        ) : (
          <DataTable
            caption={`Questions in ${exam.title}`}
            rows={attached}
            keyExtractor={(question) => question.id}
            columns={[
              {
                key: "position",
                header: "#",
                className: "w-10",
                cell: (question) => (
                  <span className="text-sm text-muted-foreground">
                    {attached.indexOf(question) + 1}
                  </span>
                ),
              },
              {
                key: "question",
                header: "Question",
                cell: (question) => <span className="font-medium">{question.question_text}</span>,
              },
              {
                key: "type",
                header: "Type",
                cell: (question) => (
                  <span className="text-sm">{questionTypeLabel(question.question_type)}</span>
                ),
              },
              {
                key: "marks",
                header: "Marks",
                cell: (question) => (
                  <span className="font-heading text-lg font-black">{question.marks}</span>
                ),
              },
              {
                key: "actions",
                header: "Actions",
                className: "w-40",
                cell: (question) => {
                  const index = attached.indexOf(question);
                  return (
                    <div className="flex items-center gap-1">
                      <TeacherCbtMoveQuestionButton
                        examId={exam.id}
                        questionId={question.id}
                        direction="up"
                        disabled={index === 0}
                      />
                      <TeacherCbtMoveQuestionButton
                        examId={exam.id}
                        questionId={question.id}
                        direction="down"
                        disabled={index === attached.length - 1}
                      />
                      <TeacherCbtQuestionFormSheet mode="edit" exam={exam} initial={question} />
                      <TeacherCbtAttachQuestionButton
                        examId={exam.id}
                        questionId={question.id}
                        attached
                      />
                      <TeacherCbtDeleteQuestionButton questionId={question.id} examId={exam.id} />
                    </div>
                  );
                },
              },
            ]}
            empty={
              <EmptyState
                title="No questions yet"
                description="Add your first question to build this exam. The total marks update as you go."
                icon={<ListChecks className="size-8" />}
              />
            }
          />
        )}
      </section>

      {!questionsError && available.length > 0 ? (
        <section className="space-y-4">
          <h2 className="font-sans font-semibold tracking-tight">Available in the question bank</h2>
          <DataTable
            caption={`Question bank for ${exam.title}`}
            rows={available}
            keyExtractor={(question) => question.id}
            columns={[
              {
                key: "question",
                header: "Question",
                cell: (question) => <span className="font-medium">{question.question_text}</span>,
              },
              {
                key: "type",
                header: "Type",
                cell: (question) => (
                  <span className="text-sm">{questionTypeLabel(question.question_type)}</span>
                ),
              },
              {
                key: "marks",
                header: "Marks",
                cell: (question) => (
                  <span className="font-heading text-lg font-black">{question.marks}</span>
                ),
              },
              {
                key: "actions",
                header: "Actions",
                className: "w-28",
                cell: (question) => (
                  <div className="flex items-center gap-2">
                    <TeacherCbtAttachQuestionButton
                      examId={exam.id}
                      questionId={question.id}
                      attached={false}
                    />
                    <TeacherCbtQuestionFormSheet mode="edit" exam={exam} initial={question} />
                  </div>
                ),
              },
            ]}
            empty={
              <EmptyState
                title="Question bank is empty"
                description="Reusable questions for this subject will appear here."
                icon={<ListChecks className="size-8" />}
              />
            }
          />
        </section>
      ) : null}

      <section className="space-y-4">
        <h2 className="font-sans font-semibold tracking-tight">Submissions</h2>
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
                Submission records do not carry an exam reference yet, so this list relies on the
                exam filter being honoured by the CBT service.
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
                      <span className="font-semibold">
                        {studentNames.get(submission.student_id)}
                      </span>
                    ) : (
                      <span className="font-mono text-xs">
                        {submission.student_id.slice(0, 8)}…
                      </span>
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
                  description="Student attempts will appear here once the exam is active."
                  icon={<MonitorCheck className="size-8" />}
                />
              }
            />
          </>
        )}
      </section>
    </div>
  );
}
