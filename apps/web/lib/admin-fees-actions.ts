"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminFeesActionResult {
  success?: boolean;
  error?: string;
}

type CreateFeeStructure = OpenAPI.fees_v1.components["schemas"]["CreateFeeStructure"];
type UpdateFeeStructure = OpenAPI.fees_v1.components["schemas"]["UpdateFeeStructure"];
type CreateInvoice = OpenAPI.fees_v1.components["schemas"]["CreateInvoice"];

function field(formData: FormData, key: string): string {
  return String((formData.get(key) as string | null) ?? "").trim();
}

/** Parse a major-unit amount ("1500" or "1500.50") into integer cents. */
function parseAmountToCents(raw: string): number | null {
  if (!raw) return null;
  if (!/^\d+(\.\d{1,2})?$/.test(raw)) return null;
  const cents = Math.round(Number(raw) * 100);
  return Number.isSafeInteger(cents) && cents >= 0 ? cents : null;
}

function parseDueDay(raw: string): number | null | undefined {
  if (!raw) return null;
  const value = Number(raw);
  if (!Number.isInteger(value) || value < 1 || value > 31) return undefined;
  return value;
}

export async function createFeeStructureAction(
  _prev: AdminFeesActionResult,
  formData: FormData,
): Promise<AdminFeesActionResult> {
  const name = field(formData, "name");
  const academicYearId = field(formData, "academic_year_id");
  const recurrence = field(formData, "recurrence");
  const target = field(formData, "target");
  if (!name) return { error: "Structure name is required." };
  if (!academicYearId) return { error: "Academic year is required." };
  if (!recurrence || !target) return { error: "Recurrence and billing scope are required." };

  const amountCents = parseAmountToCents(field(formData, "amount"));
  if (amountCents === null) {
    return { error: "Enter a valid amount (for example 1500 or 1500.50)." };
  }
  const dueDay = parseDueDay(field(formData, "due_day"));
  if (dueDay === undefined) return { error: "Due day must be a whole number between 1 and 31." };

  const body: CreateFeeStructure = {
    name,
    academic_year_id: academicYearId,
    amount_cents: amountCents,
    currency: field(formData, "currency").toUpperCase() || "GHS",
    recurrence: recurrence as CreateFeeStructure["recurrence"],
    target: target as CreateFeeStructure["target"],
    due_day: dueDay,
    description: field(formData, "description") || null,
  };

  try {
    const client = await createServerClient();
    await client.post("/api/v1/fee-structures", body);
    revalidatePath("/admin/fees");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create the fee structure." };
  }
}

export async function updateFeeStructureAction(
  id: string,
  _prev: AdminFeesActionResult,
  formData: FormData,
): Promise<AdminFeesActionResult> {
  const name = field(formData, "name");
  if (!name) return { error: "Structure name is required." };

  const amountCents = parseAmountToCents(field(formData, "amount"));
  if (amountCents === null) {
    return { error: "Enter a valid amount (for example 1500 or 1500.50)." };
  }
  const dueDay = parseDueDay(field(formData, "due_day"));
  if (dueDay === undefined) return { error: "Due day must be a whole number between 1 and 31." };

  const body: UpdateFeeStructure = {
    name,
    academic_year_id: field(formData, "academic_year_id") || undefined,
    amount_cents: amountCents,
    currency: field(formData, "currency").toUpperCase() || undefined,
    recurrence: field(formData, "recurrence") as UpdateFeeStructure["recurrence"],
    target: field(formData, "target") as UpdateFeeStructure["target"],
    due_day: dueDay,
    description: field(formData, "description") || null,
    status: field(formData, "status") as UpdateFeeStructure["status"],
  };

  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/fee-structures/${encodeURIComponent(id)}`, body);
    revalidatePath("/admin/fees");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update the fee structure." };
  }
}

export async function deleteFeeStructureAction(id: string): Promise<AdminFeesActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/fee-structures/${encodeURIComponent(id)}`);
    revalidatePath("/admin/fees");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete the fee structure." };
  }
}

export async function issueInvoiceAction(
  _prev: AdminFeesActionResult,
  formData: FormData,
): Promise<AdminFeesActionResult> {
  const feeStructureId = field(formData, "fee_structure_id");
  const studentIds = formData
    .getAll("student_ids")
    .map((value) => (typeof value === "string" ? value.trim() : ""))
    .filter(Boolean);
  if (!feeStructureId) return { error: "Choose a fee structure to bill." };
  if (studentIds.length === 0) return { error: "Choose at least one student to invoice." };

  const amountRaw = field(formData, "amount");
  let amountCents: number | undefined;
  if (amountRaw) {
    const parsed = parseAmountToCents(amountRaw);
    if (parsed === null) {
      return { error: "Enter a valid amount override (for example 1500 or 1500.50)." };
    }
    amountCents = parsed;
  }
  const dueDate = field(formData, "due_date");
  const notes = field(formData, "notes");

  try {
    const client = await createServerClient();
    for (const studentId of studentIds) {
      const body: CreateInvoice = {
        student_id: studentId,
        fee_structure_id: feeStructureId,
        ...(amountCents !== undefined ? { amount_cents: amountCents } : {}),
        ...(dueDate ? { due_date: dueDate } : {}),
        notes: notes || null,
      };
      await client.post("/api/v1/invoices", body);
    }
    revalidatePath("/admin/fees");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to issue the invoice." };
  }
}
