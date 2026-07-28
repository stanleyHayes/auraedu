import type { OpenAPI } from "@auraedu/shared-types";

export type Exam = OpenAPI.cbt_v1.components["schemas"]["Exam"];
export type Submission = OpenAPI.cbt_v1.components["schemas"]["Submission"];
/** The CBT backend is adding an exam reference to submissions; read it defensively until it lands. */
export type SubmissionRow = Submission & { exam_id?: string | null };

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
