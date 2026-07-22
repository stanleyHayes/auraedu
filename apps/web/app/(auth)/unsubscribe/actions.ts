"use server";

import { createGatewayClient } from "@auraedu/api-client";
import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";

export interface UnsubscribeResult {
  success?: boolean;
  error?: string;
}

export async function unsubscribeAction(
  _previous: UnsubscribeResult,
  formData: FormData,
): Promise<UnsubscribeResult> {
  const value = formData.get("token");
  const token = typeof value === "string" ? value.trim() : "";
  if (token.length < 32 || token.length > 1024 || /[\s\r\n]/.test(token)) {
    return { error: "This email preference link is invalid or incomplete." };
  }
  const client = createGatewayClient({
    baseUrl: gatewayInternalUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => undefined,
    getTenantCode: () => undefined,
  });
  try {
    await client.post("/api/v1/email-preferences/unsubscribe", { token });
    return { success: true };
  } catch (error) {
    const status = (error as { status?: number }).status ?? 500;
    if (status === 422) return { error: "This email preference link is invalid or has expired." };
    return { error: "Email preferences are temporarily unavailable. Please try again." };
  }
}
