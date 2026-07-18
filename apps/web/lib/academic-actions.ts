"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AcademicActionResult {
  success?: boolean;
  error?: string;
}

type CreateClass = OpenAPI.academic_v1.components["schemas"]["CreateClass"];
type CreateSubject = OpenAPI.academic_v1.components["schemas"]["CreateSubject"];

// The academic.v1 contract does not declare update schemas yet; the service
// accepts partial create payloads on PATCH (empty string clears optional text,
// null/absent leaves the field unchanged).
type UpdateClass = Partial<Pick<CreateClass, "name" | "class_teacher_id" | "capacity">>;
type UpdateSubject = Partial<CreateSubject>;

function field(formData: FormData, key: string): string {
  return String((formData.get(key) as string | null) ?? "").trim();
}

function parseCapacity(raw: string): number | null | undefined {
  if (!raw) return null;
  const value = Number(raw);
  if (!Number.isInteger(value) || value <= 0) return undefined;
  return value;
}

export async function createClassAction(
  _prev: AcademicActionResult,
  formData: FormData,
): Promise<AcademicActionResult> {
  const name = field(formData, "name");
  const academicYearId = field(formData, "academic_year_id");
  const classTeacherId = field(formData, "class_teacher_id");
  const capacity = parseCapacity(field(formData, "capacity"));

  if (!name) {
    return { error: "Class name is required." };
  }
  if (!academicYearId) {
    return { error: "Academic year is required." };
  }
  if (capacity === undefined) {
    return { error: "Capacity must be a positive whole number." };
  }

  const body: CreateClass = {
    name,
    academic_year_id: academicYearId,
    class_teacher_id: classTeacherId || null,
    capacity,
  };

  const client = await createServerClient();
  try {
    await client.post("/api/v1/classes", body);
    revalidatePath("/admin/classes");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create class." };
  }
}

export async function updateClassAction(
  id: string,
  _prev: AcademicActionResult,
  formData: FormData,
): Promise<AcademicActionResult> {
  const name = field(formData, "name");
  const classTeacherId = field(formData, "class_teacher_id");
  const capacityRaw = field(formData, "capacity");

  if (!name) {
    return { error: "Class name is required." };
  }

  const body: UpdateClass = { name, class_teacher_id: classTeacherId };
  if (capacityRaw) {
    const capacity = parseCapacity(capacityRaw);
    if (capacity === undefined) {
      return { error: "Capacity must be a positive whole number." };
    }
    body.capacity = capacity;
  }

  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/classes/${encodeURIComponent(id)}`, body);
    revalidatePath("/admin/classes");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update class." };
  }
}

export async function deleteClassAction(id: string): Promise<AcademicActionResult> {
  const client = await createServerClient();
  try {
    await client.del(`/api/v1/classes/${encodeURIComponent(id)}`);
    revalidatePath("/admin/classes");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete class." };
  }
}

export async function createSubjectAction(
  _prev: AcademicActionResult,
  formData: FormData,
): Promise<AcademicActionResult> {
  const name = field(formData, "name");
  if (!name) {
    return { error: "Subject name is required." };
  }

  const body: CreateSubject = {
    name,
    code: field(formData, "code") || null,
    description: field(formData, "description") || null,
  };

  const client = await createServerClient();
  try {
    await client.post("/api/v1/subjects", body);
    revalidatePath("/admin/subjects");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create subject." };
  }
}

export async function updateSubjectAction(
  id: string,
  _prev: AcademicActionResult,
  formData: FormData,
): Promise<AcademicActionResult> {
  const name = field(formData, "name");
  if (!name) {
    return { error: "Subject name is required." };
  }

  const body: UpdateSubject = {
    name,
    code: field(formData, "code"),
    description: field(formData, "description"),
  };

  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/subjects/${encodeURIComponent(id)}`, body);
    revalidatePath("/admin/subjects");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update subject." };
  }
}

export async function deleteSubjectAction(id: string): Promise<AcademicActionResult> {
  const client = await createServerClient();
  try {
    await client.del(`/api/v1/subjects/${encodeURIComponent(id)}`);
    revalidatePath("/admin/subjects");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete subject." };
  }
}
