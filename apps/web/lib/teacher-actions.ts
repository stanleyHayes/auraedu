"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface TeacherActionResult {
  success?: boolean;
  error?: string;
}

type BulkAttendanceRequest =
  OpenAPI.attendance_v1.components["schemas"]["BulkAttendanceRequest"];
type CreateAssignment = OpenAPI.assessment_v1.components["schemas"]["CreateAssignment"];
type UpdateAssignment = OpenAPI.assessment_v1.components["schemas"]["UpdateAssignment"];

function field(formData: FormData, key: string): string {
  return String((formData.get(key) as string | null) ?? "").trim();
}

const ATTENDANCE_STATUSES = new Set(["present", "absent", "late", "excused"]);

export async function markAttendanceBulkAction(
  _prev: TeacherActionResult,
  formData: FormData,
): Promise<TeacherActionResult> {
  const date = field(formData, "date");
  const academicYearId = field(formData, "academic_year_id");
  const classId = field(formData, "class_id");
  const subjectId = field(formData, "subject_id");

  if (!date) {
    return { error: "Date is required." };
  }
  if (!academicYearId) {
    return { error: "Academic year is required." };
  }

  // Per-student statuses arrive as `status_<student_id>` fields; absent rows
  // (e.g. an empty roster) mean there is nothing to submit.
  const records: BulkAttendanceRequest["records"] = [];
  for (const [key, value] of formData.entries()) {
    if (!key.startsWith("status_") || typeof value !== "string") continue;
    const status = value;
    if (!ATTENDANCE_STATUSES.has(status)) continue;
    records.push({
      student_id: key.slice("status_".length),
      status: status as BulkAttendanceRequest["records"][number]["status"],
    });
  }

  if (records.length === 0) {
    return { error: "No students to mark." };
  }

  const body: BulkAttendanceRequest = {
    date,
    academic_year_id: academicYearId,
    class_id: classId || null,
    subject_id: subjectId || null,
    records,
  };

  const client = await createServerClient();
  try {
    await client.post("/api/v1/attendance/bulk", body);
    revalidatePath("/teacher/attendance");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to save attendance." };
  }
}

function parseMaxScore(raw: string): number | undefined {
  if (!raw) return undefined;
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) return undefined;
  return value;
}

// The contract expects an RFC 3339 date-time; date inputs yield YYYY-MM-DD.
function toDateTime(raw: string): string | null {
  if (!raw) return null;
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed.toISOString();
}

export async function createAssignmentAction(
  _prev: TeacherActionResult,
  formData: FormData,
): Promise<TeacherActionResult> {
  const title = field(formData, "title");
  const subjectId = field(formData, "subject_id");
  const academicYearId = field(formData, "academic_year_id");
  const classId = field(formData, "class_id");
  const maxScore = parseMaxScore(field(formData, "max_score"));

  if (!title) {
    return { error: "Title is required." };
  }
  if (!subjectId) {
    return { error: "Subject is required." };
  }
  if (!academicYearId) {
    return { error: "Academic year is required." };
  }
  if (maxScore === undefined) {
    return { error: "Max score must be a positive number." };
  }

  const body: CreateAssignment = {
    title,
    instructions: field(formData, "instructions") || null,
    subject_id: subjectId,
    academic_year_id: academicYearId,
    class_ids: classId ? [classId] : [],
    due_date: toDateTime(field(formData, "due_date")),
    max_score: maxScore,
  };

  const client = await createServerClient();
  try {
    await client.post("/api/v1/assignments", body);
    revalidatePath("/teacher/assignments");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create assignment." };
  }
}

export async function updateAssignmentAction(
  id: string,
  _prev: TeacherActionResult,
  formData: FormData,
): Promise<TeacherActionResult> {
  const title = field(formData, "title");
  const classId = field(formData, "class_id");

  if (!title) {
    return { error: "Title is required." };
  }

  const body: UpdateAssignment = {
    title,
    instructions: field(formData, "instructions"),
    class_ids: classId ? [classId] : [],
    due_date: toDateTime(field(formData, "due_date")),
  };

  const maxScoreRaw = field(formData, "max_score");
  if (maxScoreRaw) {
    const maxScore = parseMaxScore(maxScoreRaw);
    if (maxScore === undefined) {
      return { error: "Max score must be a positive number." };
    }
    body.max_score = maxScore;
  }

  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/assignments/${encodeURIComponent(id)}`, body);
    revalidatePath("/teacher/assignments");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update assignment." };
  }
}

export async function publishAssignmentAction(id: string): Promise<TeacherActionResult> {
  const client = await createServerClient();
  try {
    await client.post(`/api/v1/assignments/${encodeURIComponent(id)}/publish`, {});
    revalidatePath("/teacher/assignments");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to publish assignment." };
  }
}

export async function deleteAssignmentAction(id: string): Promise<TeacherActionResult> {
  const client = await createServerClient();
  try {
    await client.del(`/api/v1/assignments/${encodeURIComponent(id)}`);
    revalidatePath("/teacher/assignments");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete assignment." };
  }
}
