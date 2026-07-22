"use server";

import { revalidatePath } from "next/cache";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClientForTenant } from "./api";

export interface FeatureFlagActionResult {
  success?: boolean;
  error?: string;
}

/**
 * Platform super-admin override of a tenant feature flag
 * (POST /api/v1/super-admin/features/{key}/override, contract tenant.v1).
 * The tenant service requires a reason and still enforces plan entitlement when
 * enabling a feature above the tenant's plan (403 plan_required).
 */
export async function overrideFeatureFlagAction(
  tenantCode: string,
  featureKey: string,
  isEnabled: boolean,
  _prev: FeatureFlagActionResult,
  formData: FormData,
): Promise<FeatureFlagActionResult> {
  const reason = String((formData.get("reason") as string | null) ?? "").trim();
  if (!reason) {
    return { error: "A reason is required for platform overrides." };
  }

  const client = await createServerClientForTenant(tenantCode);
  try {
    await client.post<OpenAPI.tenant_v1.components["schemas"]["FeatureFlag"]>(
      `/api/v1/super-admin/features/${encodeURIComponent(featureKey)}/override`,
      { tenant_code: tenantCode, is_enabled: isEnabled, reason },
    );
    revalidatePath("/superadmin/flags");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update feature flag." };
  }
}
