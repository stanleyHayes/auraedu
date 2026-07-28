"use server";

import { revalidatePath } from "next/cache";
import { ApiError } from "@auraedu/api-client";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AdminPermissionActionResult {
  success?: boolean;
  error?: string;
}

type UpdateUserRequest = OpenAPI.identity_v1.components["schemas"]["UpdateUserRequest"];

function permissionKeys(data: FormData): string[] {
  return [
    ...new Set(
      data
        .getAll("permission")
        .map((entry) => (typeof entry === "string" ? entry.trim() : ""))
        .filter((entry) => entry.length > 0),
    ),
  ];
}

/**
 * Replace a user's explicit permission grants via PATCH /api/v1/users/{id}.
 * The identity service validates the grant server-side: the actor can only grant
 * permissions they hold themselves, so a 403 here is an honest answer, not a bug.
 */
export async function updateUserPermissionsAction(
  userId: string,
  _previous: AdminPermissionActionResult,
  data: FormData,
): Promise<AdminPermissionActionResult> {
  const body: UpdateUserRequest = { permissions: permissionKeys(data) };
  try {
    const client = await createServerClient();
    await client.patch(`/api/v1/users/${encodeURIComponent(userId)}`, body);
    revalidatePath("/admin/users");
    return { success: true };
  } catch (error) {
    if (error instanceof ApiError && error.status === 403) {
      return {
        error: `${error.message} You can only grant permissions you hold yourself — ask a more privileged administrator to widen this account.`,
      };
    }
    return {
      error: error instanceof Error ? error.message : "Could not update the permission grants.",
    };
  }
}
