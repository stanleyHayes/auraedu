"use server";

import { revalidatePath } from "next/cache";
import { ApiError } from "@auraedu/api-client";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminReportActionResult {
  success?: boolean;
  error?: string;
}

type CreateReportTemplate = OpenAPI.report_v1.components["schemas"]["CreateReportTemplate"];
type UpdateReportTemplate = OpenAPI.report_v1.components["schemas"]["UpdateReportTemplate"];
type UpdateReportCard = OpenAPI.report_v1.components["schemas"]["UpdateReportCard"];

function field(formData: FormData, key: string): string {
  return String((formData.get(key) as string | null) ?? "").trim();
}

export async function createReportTemplateAction(
  _prev: AdminReportActionResult,
  formData: FormData,
): Promise<AdminReportActionResult> {
  const name = field(formData, "name");
  const academicYearId = field(formData, "academic_year_id");
  const bodyTemplate = field(formData, "body_template");
  if (!name) return { error: "Template name is required." };
  if (!academicYearId) return { error: "Academic year is required." };
  if (!bodyTemplate) return { error: "The body template is required." };

  const body: CreateReportTemplate = {
    name,
    academic_year_id: academicYearId,
    body_template: bodyTemplate,
  };
  try {
    const client = await createServerClient();
    await client.post("/api/v1/report-templates", body);
    revalidatePath("/admin/reports");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create the report template." };
  }
}

export async function updateReportTemplateAction(
  id: string,
  _prev: AdminReportActionResult,
  formData: FormData,
): Promise<AdminReportActionResult> {
  const name = field(formData, "name");
  const bodyTemplate = field(formData, "body_template");
  if (!name) return { error: "Template name is required." };
  if (!bodyTemplate) return { error: "The body template is required." };

  const body: UpdateReportTemplate = {
    name,
    academic_year_id: field(formData, "academic_year_id") || undefined,
    body_template: bodyTemplate,
    status: field(formData, "status") as UpdateReportTemplate["status"],
  };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/report-templates/${encodeURIComponent(id)}`, body);
    revalidatePath("/admin/reports");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update the report template." };
  }
}

export async function deleteReportTemplateAction(id: string): Promise<AdminReportActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/report-templates/${encodeURIComponent(id)}`);
    revalidatePath("/admin/reports");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete the report template." };
  }
}

export async function generateReportCardAction(id: string): Promise<AdminReportActionResult> {
  try {
    const client = await createServerClient();
    await client.post(`/api/v1/report-cards/${encodeURIComponent(id)}/generate`, {});
    revalidatePath("/admin/reports");
    return { success: true };
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      return { error: "This card is already generating and cannot be restarted right now." };
    }
    return { error: e instanceof Error ? e.message : "Failed to start PDF generation." };
  }
}

async function setReportCardStatus(
  id: string,
  status: UpdateReportCard["status"],
  failure: string,
): Promise<AdminReportActionResult> {
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/report-cards/${encodeURIComponent(id)}`, { status });
    revalidatePath("/admin/reports");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : failure };
  }
}

export async function publishReportCardAction(id: string): Promise<AdminReportActionResult> {
  return setReportCardStatus(id, "published", "Failed to publish the report card.");
}

export async function archiveReportCardAction(id: string): Promise<AdminReportActionResult> {
  return setReportCardStatus(id, "archived", "Failed to archive the report card.");
}
