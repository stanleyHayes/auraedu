"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface AccountActionResult {
  success?: string;
  error?: string;
}

type UpdateUserRequest = OpenAPI.identity_v1.components["schemas"]["UpdateUserRequest"];

function field(data: FormData, key: string): string {
  const value = data.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export async function updatePlatformProfileAction(
  _previous: AccountActionResult,
  data: FormData,
): Promise<AccountActionResult> {
  const userId = field(data, "user_id");
  const name = field(data, "name");
  if (!userId || name.length < 2) {
    return { error: "Enter a display name with at least two characters." };
  }

  try {
    const client = await createServerClient();
    const body: UpdateUserRequest = { name };
    await client.put(`/api/v1/users/${encodeURIComponent(userId)}`, body);
    revalidatePath("/superadmin/settings");
    return { success: "Profile updated." };
  } catch (cause) {
    return { error: cause instanceof Error ? cause.message : "Could not update your profile." };
  }
}

export async function requestPlatformPasswordResetAction(
  _previous: AccountActionResult,
  data: FormData,
): Promise<AccountActionResult> {
  const email = field(data, "email");
  if (!email.includes("@")) return { error: "Your account email is unavailable." };

  try {
    const client = await createServerClient();
    await client.post("/api/v1/auth/forgot-password", { email });
    return {
      success:
        "A secure password-reset link has been requested. In local development, check the notification service output.",
    };
  } catch (cause) {
    return {
      error: cause instanceof Error ? cause.message : "Could not request a password reset.",
    };
  }
}
