"use server";

import { revalidatePath } from "next/cache";
import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";
import type { OpenAPI } from "@auraedu/shared-types";
import { getCurrentTenantCode, getCurrentToken } from "./api";

type ImportResult = OpenAPI.student_v1.components["schemas"]["ImportResult"];

export interface AdminImportActionResult {
  success?: boolean;
  result?: ImportResult;
  error?: string;
}

const MAX_CSV_BYTES = 2 * 1024 * 1024;

async function extractErrorMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as Record<string, unknown>;
    const nested = body.error;
    if (nested && typeof nested === "object") {
      const message = (nested as Record<string, unknown>).message;
      if (typeof message === "string") return message;
    }
    if (typeof body.message === "string") return body.message;
  } catch {
    // fall through to the status line
  }
  return `Import failed with status ${response.status}.`;
}

/**
 * POST /api/v1/students/import is multipart/form-data, which the JSON-only gateway
 * client cannot encode — so this action posts the file directly with the same
 * bearer token and tenant header the client would attach.
 */
export async function importStudentsAction(
  _previous: AdminImportActionResult,
  data: FormData,
): Promise<AdminImportActionResult> {
  const entry = data.get("csv_text");
  const csvText = typeof entry === "string" ? entry : "";
  if (!csvText.trim()) return { error: "Choose a CSV file or paste its contents first." };
  if (new TextEncoder().encode(csvText).length > MAX_CSV_BYTES) {
    return { error: "The CSV is larger than 2 MB. Split it into smaller batches." };
  }
  const headerLine = csvText.trimStart().split(/\r?\n/, 1)[0]?.toLowerCase() ?? "";
  if (!headerLine.includes("first_name") || !headerLine.includes("last_name")) {
    return {
      error: "The first row must be the header row and include first_name and last_name.",
    };
  }

  const [token, tenantCode] = await Promise.all([getCurrentToken(), getCurrentTenantCode()]);
  const upload = new FormData();
  upload.append("file", new File([csvText], "students.csv", { type: "text/csv" }));

  const headers = new Headers({ accept: "application/json" });
  if (token) headers.set("authorization", `Bearer ${token}`);
  if (tenantCode) headers.set(tenantHeaderName, tenantCode);

  let response: Response;
  try {
    response = await fetch(`${gatewayInternalUrl.replace(/\/$/, "")}/api/v1/students/import`, {
      method: "POST",
      headers,
      body: upload,
    });
  } catch {
    return { error: "Could not reach the student service. Try again in a moment." };
  }

  if (!response.ok) {
    return { error: await extractErrorMessage(response) };
  }

  const result = (await response.json()) as ImportResult;
  if ((result.students_created ?? 0) > 0) revalidatePath("/admin/students");
  return { success: true, result };
}
