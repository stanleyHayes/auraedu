"use server";

import { cookies } from "next/headers";
import { createGatewayClient } from "@auraedu/api-client";
import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";
import { canonicalTenantCode } from "@/lib/tenant";

const TENANT_COOKIE = "auraedu_tenant_code";

export interface ForgotPasswordResult {
  success?: boolean;
  error?: string;
}

export async function forgotPasswordAction(
  _previous: ForgotPasswordResult,
  formData: FormData,
): Promise<ForgotPasswordResult> {
  const jar = await cookies();
  const submitted = canonicalTenantCode(formString(formData, "tenant"));
  const tenant = submitted || canonicalTenantCode(jar.get(TENANT_COOKIE)?.value);
  const email = formString(formData, "email").trim().toLowerCase();
  if (!tenant)
    return { error: "Enter your school workspace code or open your school's portal link." };
  if (email.length > 254 || !email.includes("@")) return { error: "Enter a valid email address." };

  const client = createGatewayClient({
    baseUrl: gatewayInternalUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => undefined,
    getTenantCode: () => tenant,
  });
  try {
    await client.post("/api/v1/auth/forgot-password", { email });
    jar.set(TENANT_COOKIE, tenant, {
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: 60 * 60 * 24 * 30,
    });
    return { success: true };
  } catch {
    return { error: "Password recovery is temporarily unavailable. Please try again." };
  }
}

function formString(formData: FormData, key: string): string {
  const value = formData.get(key);
  return typeof value === "string" ? value : "";
}
