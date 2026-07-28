"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";
import { QUESTION_TYPES, type Exam, type QuestionType } from "./teacher-cbt-utils";

export interface TeacherCbtActionResult {
  success?: boolean;
  error?: string;
}

type CreateExam = OpenAPI.cbt_v1.components["schemas"]["CreateExam"];
type UpdateExam = OpenAPI.cbt_v1.components["schemas"]["UpdateExam"];
type CreateQuestion = OpenAPI.cbt_v1.components["schemas"]["CreateQuestion"];
type UpdateQuestion = OpenAPI.cbt_v1.components["schemas"]["UpdateQuestion"];
type ExamStatus = NonNullable<Exam["status"]>;

function field(formData: FormData, key: string): string {
  return String((formData.get(key) as string | null) ?? "").trim();
}

// The contract expects an RFC 3339 date-time; datetime-local inputs yield YYYY-MM-DDTHH:mm.
function toDateTime(raw: string): string | null {
  if (!raw) return null;
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed.toISOString();
}

function parseDuration(raw: string): number | undefined {
  const value = Number(raw);
  if (!Number.isInteger(value) || value < 1) return undefined;
  return value;
}

function parseMarks(raw: string): number | undefined {
  const value = Number(raw);
  if (!Number.isInteger(value) || value < 1) return undefined;
  return value;
}

function revalidateCbt(examId?: string) {
  revalidatePath("/teacher/cbt");
  if (examId) revalidatePath(`/teacher/cbt/${examId}`);
}

export async function createExamAction(
  _prev: TeacherCbtActionResult,
  formData: FormData,
): Promise<TeacherCbtActionResult> {
  const title = field(formData, "title");
  const subjectId = field(formData, "subject_id");
  const academicYearId = field(formData, "academic_year_id");
  const duration = parseDuration(field(formData, "duration_minutes"));

  if (!title) return { error: "Title is required." };
  if (!subjectId) return { error: "Subject is required." };
  if (!academicYearId) return { error: "Academic year is required." };
  if (duration === undefined) return { error: "Duration must be a whole number of minutes." };

  const startAt = toDateTime(field(formData, "start_at"));
  const endAt = toDateTime(field(formData, "end_at"));
  if (startAt && endAt && endAt <= startAt) {
    return { error: "The closing time must be after the opening time." };
  }

  // Questions are authored on the exam detail page after creation.
  const body: CreateExam = {
    title,
    subject_id: subjectId,
    academic_year_id: academicYearId,
    duration_minutes: duration,
    question_ids: [],
    start_at: startAt,
    end_at: endAt,
  };

  const client = await createServerClient();
  try {
    await client.post("/api/v1/cbt/exams", body);
    revalidateCbt();
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create exam." };
  }
}

export async function updateExamAction(
  id: string,
  _prev: TeacherCbtActionResult,
  formData: FormData,
): Promise<TeacherCbtActionResult> {
  const title = field(formData, "title");
  const duration = parseDuration(field(formData, "duration_minutes"));

  if (!title) return { error: "Title is required." };
  if (duration === undefined) return { error: "Duration must be a whole number of minutes." };

  const startAt = toDateTime(field(formData, "start_at"));
  const endAt = toDateTime(field(formData, "end_at"));
  if (startAt && endAt && endAt <= startAt) {
    return { error: "The closing time must be after the opening time." };
  }

  const body: UpdateExam = {
    title,
    duration_minutes: duration,
    start_at: startAt,
    end_at: endAt,
  };

  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/cbt/exams/${encodeURIComponent(id)}`, body);
    revalidateCbt(id);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update exam." };
  }
}

const EXAM_STATUSES = new Set(["draft", "published", "active", "closed", "archived"]);

export async function setExamStatusAction(
  id: string,
  status: ExamStatus,
): Promise<TeacherCbtActionResult> {
  if (!EXAM_STATUSES.has(status)) return { error: "Unsupported exam status." };
  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/cbt/exams/${encodeURIComponent(id)}`, { status });
    revalidateCbt(id);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : `Failed to mark the exam as ${status}.` };
  }
}

export async function deleteExamAction(id: string): Promise<TeacherCbtActionResult> {
  const client = await createServerClient();
  try {
    await client.del(`/api/v1/cbt/exams/${encodeURIComponent(id)}`);
    revalidateCbt(id);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete exam." };
  }
}

/** Fetch the exam's current question ids so reorder/attach/detach never relies on stale client state. */
async function currentQuestionIds(
  client: Awaited<ReturnType<typeof createServerClient>>,
  examId: string,
): Promise<string[]> {
  const exam = await client.get<Exam>(`/api/v1/cbt/exams/${encodeURIComponent(examId)}`);
  return exam.question_ids ?? [];
}

async function replaceQuestionIds(
  client: Awaited<ReturnType<typeof createServerClient>>,
  examId: string,
  questionIds: string[],
): Promise<void> {
  await client.patch(`/api/v1/cbt/exams/${encodeURIComponent(examId)}`, {
    question_ids: questionIds,
  });
}

interface QuestionFormParseResult {
  body?: CreateQuestion;
  error?: string;
}

function parseQuestionForm(formData: FormData): QuestionFormParseResult {
  const questionText = field(formData, "question_text");
  const questionType = field(formData, "question_type");
  const correctAnswer = field(formData, "correct_answer");
  const marks = parseMarks(field(formData, "marks"));
  const subjectId = field(formData, "subject_id");
  const academicYearId = field(formData, "academic_year_id");
  const options = formData
    .getAll("option")
    .filter((value): value is string => typeof value === "string")
    .map((value) => value.trim())
    .filter((value) => value.length > 0);

  if (!questionText) return { error: "Question text is required." };
  if (!QUESTION_TYPES.includes(questionType as QuestionType)) {
    return { error: "Choose a question type." };
  }
  if (marks === undefined) return { error: "Marks must be a whole number of at least 1." };
  if (!correctAnswer) return { error: "Correct answer is required." };

  if (questionType === "multiple_choice") {
    if (options.length < 2) return { error: "Multiple choice needs at least two options." };
    if (!options.includes(correctAnswer)) {
      return { error: "The correct answer must match one of the options." };
    }
  }
  if (questionType === "true_false" && !["True", "False"].includes(correctAnswer)) {
    return { error: "Choose True or False as the correct answer." };
  }

  return {
    body: {
      academic_year_id: academicYearId,
      subject_id: subjectId,
      question_text: questionText,
      question_type: questionType as QuestionType,
      options: questionType === "multiple_choice" ? options : [],
      correct_answer: correctAnswer,
      marks,
    },
  };
}

export async function createQuestionAction(
  examId: string,
  _prev: TeacherCbtActionResult,
  formData: FormData,
): Promise<TeacherCbtActionResult> {
  const parsed = parseQuestionForm(formData);
  if (!parsed.body) return { error: parsed.error ?? "Question is invalid." };

  const client = await createServerClient();
  try {
    const question = await client.post<{ id: string }>("/api/v1/cbt/questions", parsed.body);
    // Attach the new question to the exam it was authored from.
    const ids = await currentQuestionIds(client, examId);
    if (!ids.includes(question.id)) {
      await replaceQuestionIds(client, examId, [...ids, question.id]);
    }
    revalidateCbt(examId);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create question." };
  }
}

export async function updateQuestionAction(
  questionId: string,
  examId: string,
  _prev: TeacherCbtActionResult,
  formData: FormData,
): Promise<TeacherCbtActionResult> {
  const parsed = parseQuestionForm(formData);
  if (!parsed.body) return { error: parsed.error ?? "Question is invalid." };

  const body: UpdateQuestion = {
    question_text: parsed.body.question_text,
    question_type: parsed.body.question_type,
    options: parsed.body.options,
    correct_answer: parsed.body.correct_answer,
    marks: parsed.body.marks,
  };

  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/cbt/questions/${encodeURIComponent(questionId)}`, body);
    revalidateCbt(examId);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update question." };
  }
}

export async function deleteQuestionAction(
  questionId: string,
  examId: string,
): Promise<TeacherCbtActionResult> {
  const client = await createServerClient();
  try {
    await client.del(`/api/v1/cbt/questions/${encodeURIComponent(questionId)}`);
    // Detach the deleted question from the exam so no dangling reference remains.
    const ids = await currentQuestionIds(client, examId);
    if (ids.includes(questionId)) {
      await replaceQuestionIds(
        client,
        examId,
        ids.filter((id) => id !== questionId),
      );
    }
    revalidateCbt(examId);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete question." };
  }
}

export async function attachQuestionAction(
  examId: string,
  questionId: string,
): Promise<TeacherCbtActionResult> {
  const client = await createServerClient();
  try {
    const ids = await currentQuestionIds(client, examId);
    if (!ids.includes(questionId)) {
      await replaceQuestionIds(client, examId, [...ids, questionId]);
    }
    revalidateCbt(examId);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to add the question to the exam." };
  }
}

export async function detachQuestionAction(
  examId: string,
  questionId: string,
): Promise<TeacherCbtActionResult> {
  const client = await createServerClient();
  try {
    const ids = await currentQuestionIds(client, examId);
    await replaceQuestionIds(
      client,
      examId,
      ids.filter((id) => id !== questionId),
    );
    revalidateCbt(examId);
    return { success: true };
  } catch (e) {
    return {
      error: e instanceof Error ? e.message : "Failed to remove the question from the exam.",
    };
  }
}

export async function moveQuestionAction(
  examId: string,
  questionId: string,
  direction: "up" | "down",
): Promise<TeacherCbtActionResult> {
  const client = await createServerClient();
  try {
    const ids = await currentQuestionIds(client, examId);
    const index = ids.indexOf(questionId);
    const target = direction === "up" ? index - 1 : index + 1;
    if (index === -1 || target < 0 || target >= ids.length) {
      return { error: "The question cannot move any further." };
    }
    const moved = ids[index]!;
    ids[index] = ids[target]!;
    ids[target] = moved;
    await replaceQuestionIds(client, examId, ids);
    revalidateCbt(examId);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to reorder questions." };
  }
}
