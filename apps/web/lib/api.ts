import { cookies } from "next/headers";
import { createGatewayClient, type GatewayClient } from "@auraedu/api-client";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import { getSession } from "./auth";

export async function createServerClient(): Promise<GatewayClient> {
  const jar = await cookies();
  const tenantCode = jar.get(tenantHeaderName)?.value ?? "";
  const token = jar.get("auraedu_access_token")?.value;

  return createGatewayClient({
    baseUrl: publicApiUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => token,
    getTenantCode: () => tenantCode,
  });
}

export async function getCurrentToken(): Promise<string | undefined> {
  const jar = await cookies();
  return jar.get("auraedu_access_token")?.value;
}

export async function getCurrentTenantCode(): Promise<string> {
  const jar = await cookies();
  return jar.get(tenantHeaderName)?.value ?? "";
}

export { getSession };
