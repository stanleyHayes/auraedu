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

/**
 * Like createServerClient, but pins the tenant header to an explicit tenant code.
 * Superadmin pages query tenant-scoped endpoints (e.g. GET /api/v1/features, which the
 * tenant service resolves from the X-Tenant-Code header) for an arbitrary picked tenant,
 * so the host-resolved cookie tenant must be overridden per request.
 */
export async function createServerClientForTenant(tenantCode: string): Promise<GatewayClient> {
  const jar = await cookies();
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
