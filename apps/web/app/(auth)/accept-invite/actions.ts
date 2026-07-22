"use server";

import { cookies } from "next/headers";
import { createGatewayClient } from "@auraedu/api-client";
import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";

const TENANT_COOKIE = "auraedu_tenant_code";

export interface AcceptInviteResult {
  success?: boolean;
  error?: string;
}

interface AcceptedUser {
  id: string;
  tenant_id: string;
  role: string;
}

function safeInviteError(status: number): string {
  if (status === 401 || status === 404)
    return "This invitation is invalid, expired, or has already been used.";
  if (status === 422) return "Please check your name and choose a stronger password.";
  if (status === 503)
    return "Account activation is temporarily unavailable. Your invitation is still safe—please try again.";
  return "We could not activate your account. Please try again.";
}

export async function acceptInviteAction(
  _previous: AcceptInviteResult,
  formData: FormData,
): Promise<AcceptInviteResult> {
  const token = formString(formData, "token").trim();
  const name = formString(formData, "name").trim();
  const password = formString(formData, "password");
  const confirmation = formString(formData, "password_confirmation");
  const passwordLength = Array.from(password).length;

  if (token.length < 32 || token.length > 512 || /[\s\r\n]/.test(token)) {
    return { error: "This invitation link is invalid or incomplete." };
  }
  if (name.length < 2 || name.length > 160) return { error: "Enter your full name." };
  if (passwordLength < 12 || passwordLength > 256)
    return { error: "Use between 12 and 256 characters for your password." };
  if (password !== confirmation) return { error: "The passwords do not match." };

  const client = createGatewayClient({
    baseUrl: gatewayInternalUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => undefined,
    getTenantCode: () => undefined,
  });

  try {
    const user = await client.post<AcceptedUser>(
      `/api/v1/public/invites/${encodeURIComponent(token)}/accept`,
      { name, password },
    );
    if (!user.id || !user.tenant_id || user.role !== "school_admin") {
      return { error: "The activated account did not match this school invitation." };
    }
    const jar = await cookies();
    jar.set(TENANT_COOKIE, user.tenant_id, {
      httpOnly: false,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: 60 * 60 * 24 * 30,
    });
    return { success: true };
  } catch (error) {
    const failure = error as { status?: number };
    return { error: safeInviteError(failure.status ?? 500) };
  }
}

function formString(formData: FormData, key: string): string {
  const value = formData.get(key);
  return typeof value === "string" ? value : "";
}
