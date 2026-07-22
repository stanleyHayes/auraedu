"use server";

import { cookies } from "next/headers";
import { createGatewayClient } from "@auraedu/api-client";
import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";
import { ACCESS_TOKEN_COOKIE, REFRESH_TOKEN_COOKIE, USER_COOKIE } from "@/lib/auth";
import { canonicalTenantCode } from "@/lib/tenant";

const TENANT_COOKIE = "auraedu_tenant_code";
export interface ResetPasswordResult {
  success?: boolean;
  error?: string;
}

export async function resetPasswordAction(
  _previous: ResetPasswordResult,
  formData: FormData,
): Promise<ResetPasswordResult> {
  const jar = await cookies();
  const tenant =
    canonicalTenantCode(formString(formData, "tenant")) ||
    canonicalTenantCode(jar.get(TENANT_COOKIE)?.value);
  const token = formString(formData, "token").trim();
  const password = formString(formData, "password");
  const confirmation = formString(formData, "password_confirmation");
  const passwordLength = Array.from(password).length;
  if (!tenant) return { error: "This recovery link is missing its school workspace." };
  if (token.length < 32 || token.length > 512 || /[\s\r\n]/.test(token))
    return { error: "This recovery link is invalid or expired." };
  if (passwordLength < 12 || passwordLength > 256)
    return { error: "Use between 12 and 256 characters for your new password." };
  if (password !== confirmation) return { error: "The passwords do not match." };

  const client = createGatewayClient({
    baseUrl: gatewayInternalUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => undefined,
    getTenantCode: () => tenant,
  });
  try {
    await client.post("/api/v1/auth/reset-password", { token, new_password: password });
    jar.set(TENANT_COOKIE, tenant, {
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: 60 * 60 * 24 * 30,
    });
    jar.delete(ACCESS_TOKEN_COOKIE);
    jar.delete(REFRESH_TOKEN_COOKIE);
    jar.delete(USER_COOKIE);
    return { success: true };
  } catch (error) {
    const status = (error as { status?: number }).status;
    return {
      error:
        status === 401
          ? "This recovery link is invalid, expired, or has already been used."
          : "We could not reset your password. Please try again.",
    };
  }
}

function formString(formData: FormData, key: string): string {
  const value = formData.get(key);
  return typeof value === "string" ? value : "";
}
