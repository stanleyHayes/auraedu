import { cookies } from "next/headers";
import { createGatewayClient, type GatewayClient } from "@auraedu/api-client";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import { ACCESS_TOKEN_COOKIE, getSession } from "./auth";

const TENANT_COOKIE = "auraedu_tenant_code";

export async function createServerClient(): Promise<GatewayClient> {
  const jar = await cookies();
  const tenantCode = jar.get(TENANT_COOKIE)?.value ?? "";
  const token = jar.get(ACCESS_TOKEN_COOKIE)?.value;

  return createGatewayClient({
    baseUrl: publicApiUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => token,
    getTenantCode: () => tenantCode,
  });
}

export async function getCurrentToken(): Promise<string | undefined> {
  const jar = await cookies();
  return jar.get(ACCESS_TOKEN_COOKIE)?.value;
}

export async function getCurrentTenantCode(): Promise<string> {
  const jar = await cookies();
  return jar.get(TENANT_COOKIE)?.value ?? "";
}

export { getSession };
