"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminGradingActionResult {
  success?: boolean;
  error?: string;
}

type GradeRange = OpenAPI.academic_v1.components["schemas"]["GradeRange"];
type CreateGradingScale = OpenAPI.academic_v1.components["schemas"]["CreateGradingScale"];
type UpdateGradingScale = OpenAPI.academic_v1.components["schemas"]["UpdateGradingScale"];

function value(data: FormData, key: string): string {
  const entry = data.get(key);
  return typeof entry === "string" ? entry.trim() : "";
}

function texts(data: FormData, key: string): string[] {
  return data.getAll(key).map((entry) => (typeof entry === "string" ? entry.trim() : ""));
}

/**
 * Bands arrive as parallel repeated fields (band_grade[], band_min[], …) from the
 * dynamic form rows — the same convention the journey builder uses for steps.
 */
function readRanges(data: FormData): { ranges: GradeRange[] } | { error: string } {
  const grades = texts(data, "band_grade");
  const mins = texts(data, "band_min");
  const maxes = texts(data, "band_max");
  const remarks = texts(data, "band_remark");
  if (
    grades.length === 0 ||
    ![mins.length, maxes.length, remarks.length].every((length) => length === grades.length)
  ) {
    return { error: "Add at least one grade band." };
  }
  const ranges: GradeRange[] = [];
  for (let index = 0; index < grades.length; index += 1) {
    const grade = grades[index];
    const min = Number(mins[index]);
    const max = Number(maxes[index]);
    if (!grade) return { error: `Band ${index + 1}: the grade label is required.` };
    if (!Number.isFinite(min) || !Number.isFinite(max)) {
      return { error: `Band ${index + 1}: min and max scores are required.` };
    }
    if (min < 0 || max > 100 || min > max) {
      return {
        error: `Band ${index + 1}: scores must sit within 0–100 and min must not exceed max.`,
      };
    }
    const remark = remarks[index];
    ranges.push({ grade, min, max, ...(remark ? { remark } : {}) });
  }
  return { ranges };
}

export async function createGradingScaleAction(
  _previous: AdminGradingActionResult,
  data: FormData,
): Promise<AdminGradingActionResult> {
  const name = value(data, "name");
  if (!name) return { error: "Name the grading scale." };
  const parsed = readRanges(data);
  if ("error" in parsed) return { error: parsed.error };
  const body: CreateGradingScale = { name, ranges: parsed.ranges };
  try {
    const client = await createServerClient();
    await client.post("/api/v1/grading-scales", body);
    revalidatePath("/admin/grading");
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not create the grading scale.",
    };
  }
}

export async function updateGradingScaleAction(
  scaleId: string,
  _previous: AdminGradingActionResult,
  data: FormData,
): Promise<AdminGradingActionResult> {
  const name = value(data, "name");
  if (!name) return { error: "Name the grading scale." };
  const parsed = readRanges(data);
  if ("error" in parsed) return { error: parsed.error };
  const body: UpdateGradingScale = { name, ranges: parsed.ranges };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/grading-scales/${encodeURIComponent(scaleId)}`, body);
    revalidatePath("/admin/grading");
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not update the grading scale.",
    };
  }
}

export async function deleteGradingScaleAction(scaleId: string): Promise<AdminGradingActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/grading-scales/${encodeURIComponent(scaleId)}`);
    revalidatePath("/admin/grading");
    return { success: true };
  } catch (error) {
    return {
      error:
        error instanceof Error
          ? error.message
          : "Could not delete the grading scale. Assessments using it may need attention first.",
    };
  }
}
