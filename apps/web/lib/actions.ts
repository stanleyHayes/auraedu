"use server";

import { cookies } from "next/headers";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { createGatewayClient } from "@auraedu/api-client";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import {
  ACCESS_TOKEN_COOKIE,
  REFRESH_TOKEN_COOKIE,
  USER_COOKIE,
} from "./auth";
import { createServerClient } from "./api";

const TENANT_COOKIE = "auraedu_tenant_code";

export async function logoutAction() {
  const jar = await cookies();
  const tenantCode = jar.get(TENANT_COOKIE)?.value ?? "";
  const refreshToken = jar.get(REFRESH_TOKEN_COOKIE)?.value;

  if (refreshToken) {
    const client = createGatewayClient({
      baseUrl: publicApiUrl,
      tenantHeader: tenantHeaderName,
      getToken: () => undefined,
      getTenantCode: () => tenantCode,
    });
    try {
      await client.post("/api/v1/auth/logout", { refresh_token: refreshToken });
    } catch {
      // Best-effort revocation; clear cookies regardless.
    }
  }

  jar.delete(ACCESS_TOKEN_COOKIE);
  jar.delete(REFRESH_TOKEN_COOKIE);
  jar.delete(USER_COOKIE);
  redirect("/login");
}

export interface ActionResult {
  success?: boolean;
  error?: string;
}

export async function recordAttendance(
  _prev: ActionResult | undefined,
  formData: FormData,
): Promise<ActionResult> {
  const studentId = String((formData.get("student_id") as string | null) ?? "").trim();
  const academicYearId = String((formData.get("academic_year_id") as string | null) ?? "").trim();
  const date = String((formData.get("date") as string | null) ?? "").trim();
  const status = String((formData.get("status") as string | null) ?? "").trim();

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

export async function recordScore(
  _prev: ActionResult | undefined,
  formData: FormData,
): Promise<ActionResult> {
  const assessmentId = String((formData.get("assessment_id") as string | null) ?? "").trim();
  const studentId = String((formData.get("student_id") as string | null) ?? "").trim();
  const scoreRaw = String((formData.get("score") as string | null) ?? "").trim();
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
