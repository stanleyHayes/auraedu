"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminUserActionResult {
  success?: boolean;
  error?: string;
}

type InviteUserRequest = OpenAPI.identity_v1.components["schemas"]["InviteUserRequest"];
type RoleAssignmentRequest = OpenAPI.identity_v1.components["schemas"]["RoleAssignmentRequest"];

function value(data: FormData, key: string): string {
  const entry = data.get(key);
  return typeof entry === "string" ? entry.trim() : "";
}

export async function inviteUserAction(
  _previous: AdminUserActionResult,
  data: FormData,
): Promise<AdminUserActionResult> {
  const email = value(data, "email");
  const role = value(data, "role");
  if (!email?.includes("@")) return { error: "A valid email address is required." };
  if (!role) return { error: "Choose the role the invite grants." };
  const body: InviteUserRequest = { email, role };
  try {
    const client = await createServerClient();
    await client.post("/api/v1/users/invites", body);
    revalidatePath("/admin/users");
    return { success: true };
  } catch (error) {
    return {
      error:
        error instanceof Error
          ? error.message
          : "Could not queue the invitation. The delivery service may be unavailable.",
    };
  }
}

export async function assignRoleAction(
  userId: string,
  _previous: AdminUserActionResult,
  data: FormData,
): Promise<AdminUserActionResult> {
  const role = value(data, "role");
  if (!role) return { error: "Choose a role to assign." };
  const body: RoleAssignmentRequest = { role };
  try {
    const client = await createServerClient();
    await client.post(`/api/v1/users/${encodeURIComponent(userId)}/roles`, body);
    revalidatePath("/admin/users");
    return { success: true };
  } catch (error) {
    return {
      error:
        error instanceof Error
          ? error.message
          : "Could not reassign the role. You can only grant roles and permissions you hold yourself.",
    };
  }
}

export async function deactivateUserAction(userId: string): Promise<AdminUserActionResult> {
  try {
    const client = await createServerClient();
    await client.del(`/api/v1/users/${encodeURIComponent(userId)}`);
    revalidatePath("/admin/users");
    return { success: true };
  } catch (error) {
    return {
      error:
        error instanceof Error
          ? error.message
          : "Could not deactivate the account. It may already be removed.",
    };
  }
}
