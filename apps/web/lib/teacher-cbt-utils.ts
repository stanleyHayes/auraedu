import type { OpenAPI } from "@auraedu/shared-types";

export type Exam = OpenAPI.cbt_v1.components["schemas"]["Exam"];
export type ExamList = OpenAPI.cbt_v1.components["schemas"]["ExamList"];
export type Question = OpenAPI.cbt_v1.components["schemas"]["Question"];
export type QuestionList = OpenAPI.cbt_v1.components["schemas"]["QuestionList"];
export type Submission = OpenAPI.cbt_v1.components["schemas"]["Submission"];
export type SubmissionList = OpenAPI.cbt_v1.components["schemas"]["SubmissionList"];
/** The CBT backend is adding an exam reference to submissions; read it defensively until it lands. */
export type SubmissionRow = Submission & { exam_id?: string | null };

export const QUESTION_TYPES = ["multiple_choice", "true_false", "short_answer"] as const;
export type QuestionType = (typeof QUESTION_TYPES)[number];

export function questionTypeLabel(type: Question["question_type"]): string {
  switch (type) {
    case "multiple_choice":
      return "Multiple choice";
    case "true_false":
      return "True / false";
    case "short_answer":
      return "Short answer";
    default:
      return "Unknown";
  }
}

export function examStatusStyle(status: Exam["status"]) {
  switch (status) {
    case "active":
      return "bg-emerald-50 text-emerald-800";
    case "published":
      return "bg-blue-50 text-blue-800";
    case "draft":
      return "bg-amber-50 text-amber-800";
    default:
      return "bg-muted text-muted-foreground";
  }
}

export function formatExamWindow(exam: Exam) {
  if (!exam.start_at && !exam.end_at) return "Not scheduled";
  const fmt = new Intl.DateTimeFormat("en-GH", { dateStyle: "medium", timeStyle: "short" });
  const start = exam.start_at ? fmt.format(new Date(exam.start_at)) : "any time";
  const end = exam.end_at ? fmt.format(new Date(exam.end_at)) : "open";
  return `${start} → ${end}`;
}

export function submissionStatusStyle(status: Submission["status"]) {
  switch (status) {
    case "graded":
      return "bg-emerald-50 text-emerald-800";
    case "submitted":
      return "bg-blue-50 text-blue-800";
    default:
      return "bg-amber-50 text-amber-800";
  }
}

/** Convert an ISO date-time to the `YYYY-MM-DDTHH:mm` shape datetime-local inputs expect. */
export function toDateTimeLocalValue(value: string | null | undefined): string {
  if (!value) return "";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())}T${pad(
    parsed.getHours(),
  )}:${pad(parsed.getMinutes())}`;
}

export function sumMarks(questions: Question[]): number {
  return questions.reduce((total, question) => total + (question.marks ?? 0), 0);
}

/** Preserve the exam's question order; bank questions not attached yet come last, alphabetically. */
export function orderExamQuestions(exam: Exam, bank: Question[]): Question[] {
  const byId = new Map(bank.map((question) => [question.id, question]));
  const attached: Question[] = [];
  for (const id of exam.question_ids ?? []) {
    const question = byId.get(id);
    if (question) attached.push(question);
  }
  return attached;
}
