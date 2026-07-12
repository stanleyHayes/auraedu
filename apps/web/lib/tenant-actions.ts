"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "./api";

export interface TenantActionResult {
  success?: boolean;
  error?: string;
}

const CODE_PATTERN = /^[a-z0-9-]{2,50}$/;

function parseBranding(formData: FormData): OpenAPI.tenant_v1.components["schemas"]["Branding"] {
  const primary = String((formData.get("brand_primary") as string | null) ?? "").trim();
  const secondary = String((formData.get("brand_secondary") as string | null) ?? "").trim();
  const logo = String((formData.get("logo_url") as string | null) ?? "").trim();
  return {
    brand: {
      primary: primary || "#C6402F",
      secondary: secondary || null,
    },
    logo_url: logo || null,
  };
}

function parseSettings(formData: FormData): OpenAPI.tenant_v1.components["schemas"]["Settings"] {
  const academicYearRaw = (formData.get("academic_year_start_month") as string | null) ?? "";
  const academicYear = academicYearRaw ? Number(academicYearRaw) : undefined;
  return {
    locale: String((formData.get("locale") as string | null) ?? "").trim() || undefined,
    timezone: String((formData.get("timezone") as string | null) ?? "").trim() || undefined,
    date_format: String((formData.get("date_format") as string | null) ?? "").trim() || undefined,
    academic_year_start_month:
      academicYear && !Number.isNaN(academicYear) && academicYear >= 1 && academicYear <= 12
        ? academicYear
        : undefined,
    primary_contact_email:
      String((formData.get("primary_contact_email") as string | null) ?? "").trim() || null,
  };
}

export async function createTenantAction(
  _prev: TenantActionResult,
  formData: FormData,
): Promise<TenantActionResult> {
  const tenantCode = String((formData.get("tenant_code") as string | null) ?? "").trim().toLowerCase();
  const name = String((formData.get("name") as string | null) ?? "").trim();

  if (!CODE_PATTERN.test(tenantCode)) {
    return { error: "Tenant code must be 2–50 lowercase letters, numbers, or hyphens." };
  }
  if (!name) {
    return { error: "School name is required." };
  }

  const short = String((formData.get("short") as string | null) ?? "").trim() || undefined;
  const status = (formData.get("status") as string | null) ?? "active";
  const plan = (formData.get("plan") as string | null) ?? "starter";
  const domain = String((formData.get("domain") as string | null) ?? "").trim() || null;

  const body: OpenAPI.tenant_v1.components["schemas"]["TenantCreate"] = {
    tenant_code: tenantCode,
    name,
    short,
    status: status as OpenAPI.tenant_v1.components["schemas"]["Tenant"]["status"],
    plan: plan as OpenAPI.tenant_v1.components["schemas"]["Tenant"]["plan"],
    domain,
    branding: parseBranding(formData),
  };

  const client = await createServerClient();
  try {
    await client.post("/api/v1/tenants", body);
    revalidatePath("/superadmin/tenants");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to create school." };
  }
}

export async function updateTenantAction(
  code: string,
  _prev: TenantActionResult,
  formData: FormData,
): Promise<TenantActionResult> {
  const name = String((formData.get("name") as string | null) ?? "").trim();
  if (!name) {
    return { error: "School name is required." };
  }

  const short = String((formData.get("short") as string | null) ?? "").trim() || null;
  const status = (formData.get("status") as string | null) ?? undefined;
  const plan = (formData.get("plan") as string | null) ?? undefined;
  const domain = String((formData.get("domain") as string | null) ?? "").trim() || null;

  const body: OpenAPI.tenant_v1.components["schemas"]["TenantUpdate"] = {
    name,
    short,
    status: status as OpenAPI.tenant_v1.components["schemas"]["Tenant"]["status"] | undefined,
    plan: plan as OpenAPI.tenant_v1.components["schemas"]["Tenant"]["plan"] | undefined,
    domain,
    branding: parseBranding(formData),
  };

  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/tenants/${encodeURIComponent(code)}`, body);
    revalidatePath("/superadmin/tenants");
    revalidatePath(`/superadmin/tenants/${code}`);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update school." };
  }
}

export async function deleteTenantAction(code: string): Promise<TenantActionResult> {
  const client = await createServerClient();
  try {
    await client.del(`/api/v1/tenants/${encodeURIComponent(code)}`);
    revalidatePath("/superadmin/tenants");
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to delete school." };
  }
}

export async function updateTenantSettingsAction(
  code: string,
  _prev: TenantActionResult,
  formData: FormData,
): Promise<TenantActionResult> {
  const body = parseSettings(formData);

  const client = await createServerClient();
  try {
    await client.patch(`/api/v1/tenants/${encodeURIComponent(code)}/settings`, body);
    revalidatePath(`/superadmin/tenants/${code}`);
    return { success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to update settings." };
  }
}

export async function requestSignedUploadAction(
  fileName: string,
  folder: string,
  resourceType: "image" | "raw" | "video" = "image",
): Promise<OpenAPI.file_v1.components["schemas"]["SignedUploadResponse"] & { error?: string }> {
  const client = await createServerClient();
  try {
    const res = await client.post<OpenAPI.file_v1.components["schemas"]["SignedUploadResponse"]>(
      "/api/v1/uploads/signed",
      { folder, file_name: fileName, resource_type: resourceType },
    );
    return res;
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to request upload." } as OpenAPI.file_v1.components["schemas"]["SignedUploadResponse"] & { error: string };
  }
}

export async function completeUploadAction(
  fileId: string,
  secureUrl: string,
  publicId: string,
  sizeBytes: number,
  contentType: string,
): Promise<{ secure_url?: string; error?: string }> {
  const client = await createServerClient();
  try {
    const res = await client.post<{ secure_url?: string }>(
      `/api/v1/files/${encodeURIComponent(fileId)}/complete`,
      { secure_url: secureUrl, public_id: publicId, size_bytes: sizeBytes, content_type: contentType },
    );
    return { secure_url: res.secure_url ?? secureUrl };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Failed to complete upload." };
  }
}

export async function navigateToTenants() {
  await Promise.resolve();
  redirect("/superadmin/tenants");
}
