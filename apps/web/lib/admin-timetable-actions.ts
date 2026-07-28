"use server";

import { revalidatePath } from "next/cache";
import { ApiError } from "@auraedu/api-client";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminTimetableActionResult {
  success?: boolean;
  error?: string;
}

type CreateTimetableEntry = OpenAPI.academic_v1.components["schemas"]["CreateTimetableEntry"];
type UpdateTimetableEntry = OpenAPI.academic_v1.components["schemas"]["UpdateTimetableEntry"];

const TIME_PATTERN = /^([01][0-9]|2[0-3]):[0-5][0-9]$/;
const CONFLICT_MESSAGE =
  "This slot overlaps an existing period for the class or teacher. Pick a different time, day, room or teacher.";

function field(formData: FormData, key: string): string {
  return String((formData.get(key) as string | null) ?? "").trim();
}

function isConflict(error: unknown): boolean {
  return error instanceof ApiError && error.status === 409;
}

function parseWeekday(raw: string): number | null {
  const value = Number(raw);
  return Number.isInteger(value) && value >= 1 && value <= 7 ? value : null;
}

function validateTimes(startTime: string, endTime: string): string | null {
  if (!TIME_PATTERN.test(startTime) || !TIME_PATTERN.test(endTime)) {
    return "Enter start and end times in 24-hour HH:MM format.";
  }
  if (endTime <= startTime) {
    return "The end time must be after the start time.";
  }
  return null;
}

export async function createTimetableEntryAction(
  _prev: AdminTimetableActionResult,
  formData: FormData,
): Promise<AdminTimetableActionResult> {
  const classId = field(formData, "class_id");
  const termId = field(formData, "term_id");
  const subjectId = field(formData, "subject_id");
  if (!classId || !termId || !subjectId) {
    return { error: "Class, term and subject are required." };
  }
  const weekday = parseWeekday(field(formData, "weekday"));
  if (weekday === null) return { error: "Choose a day of the week." };

  const startTime = field(formData, "start_time");
  const endTime = field(formData, "end_time");
  const timeError = validateTimes(startTime, endTime);
  if (timeError) return { error: timeError };

  const body: CreateTimetableEntry = {
    class_id: classId,
    term_id: termId,
    subject_id: subjectId,
    teacher_id: field(formData, "teacher_id") || null,
    weekday,
    start_time: startTime,
    end_time: endTime,
    room: field(formData, "room") || null,
  };

  try {
    const client = await createServerClient();
    await client.post("/api/v1/timetable", body);
    revalidatePath("/admin/timetable");
    return { success: true };
  } catch (e) {
    if (isConflict(e)) return { error: CONFLICT_MESSAGE };
    return { error: e instanceof Error ? e.message : "Failed to create the timetable entry." };
  }
}

export async function updateTimetableEntryAction(
  id: string,
  _prev: AdminTimetableActionResult,
  formData: FormData,
): Promise<AdminTimetableActionResult> {
  const weekday = parseWeekday(field(formData, "weekday"));
  if (weekday === null) return { error: "Choose a day of the week." };

  const startTime = field(formData, "start_time");
  const endTime = field(formData, "end_time");
  const timeError = validateTimes(startTime, endTime);
  if (timeError) return { error: timeError };

  const body: UpdateTimetableEntry = {
    teacher_id: field(formData, "teacher_id") || null,
    weekday,
    start_time: startTime,
    end_time: endTime,
    room: field(formData, "room") || null,
    status: field(formData, "status") as UpdateTimetableEntry["status"],
  };

  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/timetable/${encodeURIComponent(id)}`, body);
    revalidatePath("/admin/timetable");
    return { success: true };
  } catch (e) {
    if (isConflict(e)) return { error: CONFLICT_MESSAGE };
    return { error: e instanceof Error ? e.message : "Failed to update the timetable entry." };
  }
}

export async function deleteTimetableEntryAction(id: string): Promise<AdminTimetableActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/timetable/${encodeURIComponent(id)}`);
    revalidatePath("/admin/timetable");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete the timetable entry." };
  }
}
