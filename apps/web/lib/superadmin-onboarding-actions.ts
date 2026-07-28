"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

type OnboardingRequest = OpenAPI.tenant_v1.components["schemas"]["OnboardingRequest"];

export interface OnboardingActionResult {
  success?: boolean;
  error?: string;
}

const TENANT_CODE_PATTERN = /^[a-z0-9-]{2,50}$/;

/**
 * Approve an onboarding request and provision its tenant
 * (POST /api/v1/super-admin/onboarding-requests/{request_id}/approve, contract tenant.v1).
 * The contract requires the tenant_code the new school will claim.
 */
export async function approveOnboardingRequestAction(
  requestId: string,
  _prev: OnboardingActionResult,
  formData: FormData,
): Promise<OnboardingActionResult> {
  const tenantCode = String((formData.get("tenant_code") as string | null) ?? "")
    .trim()
    .toLowerCase();
  if (!TENANT_CODE_PATTERN.test(tenantCode)) {
    return { error: "Tenant code must be 2–50 lowercase letters, numbers, or hyphens." };
  }

  const client = await createServerClient();
  try {
    await client.post<OnboardingRequest>(
      `/api/v1/super-admin/onboarding-requests/${encodeURIComponent(requestId)}/approve`,
      { tenant_code: tenantCode },
    );
    revalidatePath("/superadmin/onboarding");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to approve the request." };
  }
}

/**
 * Reject an onboarding request
 * (POST /api/v1/super-admin/onboarding-requests/{request_id}/reject, contract tenant.v1).
 * The contract mandates a reason of 3–500 characters.
 */
export async function rejectOnboardingRequestAction(
  requestId: string,
  _prev: OnboardingActionResult,
  formData: FormData,
): Promise<OnboardingActionResult> {
  const reason = String((formData.get("reason") as string | null) ?? "").trim();
  if (reason.length < 3) {
    return { error: "A rejection reason of at least 3 characters is required." };
  }
  if (reason.length > 500) {
    return { error: "The rejection reason must be 500 characters or fewer." };
  }

  const client = await createServerClient();
  try {
    await client.post<OnboardingRequest>(
      `/api/v1/super-admin/onboarding-requests/${encodeURIComponent(requestId)}/reject`,
      { reason },
    );
    revalidatePath("/superadmin/onboarding");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to reject the request." };
  }
}
