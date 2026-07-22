"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface StudentActionResult {
  success?: boolean;
  error?: string;
}
type CreateStudent = OpenAPI.student_v1.components["schemas"]["CreateStudent"];
type UpdateStudent = OpenAPI.student_v1.components["schemas"]["UpdateStudent"];

function value(data: FormData, key: string): string {
  const entry = data.get(key);
  return typeof entry === "string" ? entry.trim() : "";
}

export async function createStudentAction(
  _previous: StudentActionResult,
  data: FormData,
): Promise<StudentActionResult> {
  const firstName = value(data, "first_name");
  const lastName = value(data, "last_name");
  if (!firstName || !lastName) return { error: "First and last name are required." };
  const gender = value(data, "gender");
  const body: CreateStudent = {
    first_name: firstName,
    last_name: lastName,
    date_of_birth: value(data, "date_of_birth") || null,
    gender: (gender || null) as CreateStudent["gender"],
    class_id: value(data, "class_id") || null,
    academic_year_id: value(data, "academic_year_id") || null,
    user_id: value(data, "user_id") || null,
  };
  if (Boolean(body.class_id) !== Boolean(body.academic_year_id)) {
    return { error: "Choose both a class and academic year to create the initial enrolment." };
  }
  try {
    const client = await createServerClient();
    await client.post("/api/v1/students", body);
    revalidatePath("/admin/students");
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not create the student record.",
    };
  }
}

export async function updateStudentAction(
  studentId: string,
  _previous: StudentActionResult,
  data: FormData,
): Promise<StudentActionResult> {
  const body: UpdateStudent = {
    first_name: value(data, "first_name"),
    last_name: value(data, "last_name"),
    status: value(data, "status") as UpdateStudent["status"],
    user_id: value(data, "user_id") || null,
  };
  if (!body.first_name || !body.last_name) return { error: "First and last name are required." };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/students/${encodeURIComponent(studentId)}`, body);
    revalidatePath("/admin/students");
    return { success: true };
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : "Could not update the student record.",
    };
  }
}
