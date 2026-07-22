"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface StaffActionResult {
  success?: boolean;
  error?: string;
}

type CreateAssignment = OpenAPI.staff_v1.components["schemas"]["CreateStaffAssignment"];
type CreateStaff = OpenAPI.staff_v1.components["schemas"]["CreateStaff"];
type UpdateStaff = OpenAPI.staff_v1.components["schemas"]["UpdateStaff"];

function value(formData: FormData, key: string): string {
  const entry = formData.get(key);
  return typeof entry === "string" ? entry.trim() : "";
}

function staffBody(formData: FormData): CreateStaff {
  return {
    first_name: value(formData, "first_name"),
    last_name: value(formData, "last_name"),
    staff_type: value(formData, "staff_type") as CreateStaff["staff_type"],
    email: value(formData, "email") || null,
    user_id: value(formData, "user_id") || null,
  };
}

function validateStaff(body: CreateStaff): string | null {
  if (!body.first_name || !body.last_name) return "First and last name are required.";
  if (body.staff_type !== "teacher" && body.staff_type !== "non_teaching") {
    return "Choose a valid staff type.";
  }
  return null;
}

export async function createStaffAction(
  _previous: StaffActionResult,
  formData: FormData,
): Promise<StaffActionResult> {
  const body = staffBody(formData);
  const validation = validateStaff(body);
  if (validation) return { error: validation };
  try {
    const client = await createServerClient();
    await client.post("/api/v1/staff", body);
    revalidatePath("/admin/staff");
    return { success: true };
  } catch (error) {
    return { error: error instanceof Error ? error.message : "Could not create the staff record." };
  }
}

export async function updateStaffAction(
  staffId: string,
  _previous: StaffActionResult,
  formData: FormData,
): Promise<StaffActionResult> {
  const base = staffBody(formData);
  const validation = validateStaff(base);
  if (validation) return { error: validation };
  const body: UpdateStaff = { ...base, status: value(formData, "status") as UpdateStaff["status"] };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/staff/${encodeURIComponent(staffId)}`, body);
    revalidatePath("/admin/staff");
    return { success: true };
  } catch (error) {
    return { error: error instanceof Error ? error.message : "Could not update the staff record." };
  }
}

export async function createStaffAssignmentAction(
  staffId: string,
  _previous: StaffActionResult,
  formData: FormData,
): Promise<StaffActionResult> {
  const classId = value(formData, "class_id");
  if (!classId) return { error: "Choose a class before assigning this teacher." };

  const body: CreateAssignment = {
    class_id: classId,
    subject_id: value(formData, "subject_id") || null,
    role: value(formData, "role") || null,
  };

  try {
    const client = await createServerClient();
    await client.post(`/api/v1/staff/${encodeURIComponent(staffId)}/assignments`, body);
    revalidatePath("/admin/staff");
    return { success: true };
  } catch (error) {
    return { error: error instanceof Error ? error.message : "Could not save the assignment." };
  }
}

export async function deleteStaffAssignmentAction(
  staffId: string,
  assignmentId: string,
): Promise<StaffActionResult> {
  try {
    const client = await createServerClient();
    await client.del(
      `/api/v1/staff/${encodeURIComponent(staffId)}/assignments/${encodeURIComponent(assignmentId)}`,
    );
    revalidatePath("/admin/staff");
    return { success: true };
  } catch (error) {
    return { error: error instanceof Error ? error.message : "Could not remove the assignment." };
  }
}
