"use server";

import { cookies } from "next/headers";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { createServerClient } from "./api";

export async function logoutAction() {
  const jar = await cookies();
  jar.delete("auraedu_access_token");
  jar.delete("auraedu_user");
  redirect("/login");
}

export interface ActionResult {
  success?: boolean;
  error?: string;
}

export async function recordAttendance(_prev: ActionResult | undefined, formData: FormData): Promise<ActionResult> {
  const studentId = String(formData.get("student_id") ?? "").trim();
  const academicYearId = String(formData.get("academic_year_id") ?? "").trim();
  const date = String(formData.get("date") ?? "").trim();
  const status = String(formData.get("status") ?? "").trim();

  if (!studentId || !academicYearId || !date || !status) {
    return { error: "All fields are required." };
  }

  const client = await createServerClient();
  try {
    await client.post("/api/v1/attendance", {
      student_id: studentId,
      academic_year_id: academicYearId,
      date,
      status,
    });
    revalidatePath("/teacher/attendance");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to record attendance." };
  }
}

export async function recordScore(_prev: ActionResult | undefined, formData: FormData): Promise<ActionResult> {
  const assessmentId = String(formData.get("assessment_id") ?? "").trim();
  const studentId = String(formData.get("student_id") ?? "").trim();
  const scoreRaw = String(formData.get("score") ?? "").trim();
  const score = Number(scoreRaw);

  if (!assessmentId || !studentId || !scoreRaw || Number.isNaN(score)) {
    return { error: "All fields are required and score must be a number." };
  }

  const client = await createServerClient();
  try {
    await client.post(`/api/v1/assessments/${assessmentId}/scores`, {
      student_id: studentId,
      score,
    });
    revalidatePath("/teacher/scores");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to record score." };
  }
}
