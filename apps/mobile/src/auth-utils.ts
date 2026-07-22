export const mobileRoles = ["teacher", "parent", "student"] as const;
export type MobileRole = (typeof mobileRoles)[number];

export interface MobileSession {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
  user: {
    id: string;
    name: string;
    email: string;
    role: MobileRole;
    tenant_id: string;
    permissions: string[];
  };
}

export interface TokenPairPayload {
  access_token?: unknown;
  refresh_token?: unknown;
  expires_at?: unknown;
  user?: unknown;
  message?: unknown;
}

export interface TenantBranding {
  code: string;
  name: string;
  short: string;
  status: "active" | "suspended" | "onboarding";
  logoUrl?: string;
  primary: string;
  secondary?: string;
}

export interface TenantBrandingPayload {
  tenant_code?: string;
  name?: string;
  short?: string;
  status?: TenantBranding["status"];
  branding?: { logo_url?: string | null; brand?: { primary?: string; secondary?: string | null } };
  message?: string;
}

const allowedRoles = new Set<string>(mobileRoles);
const hexColor = /^#[0-9a-f]{6}$/i;

export function normalizeGatewayApiUrl(url: string, requireHttps = true): string {
  const normalized = url.trim().replace(/\/+$/, "");
  let parsed: URL;
  try {
    parsed = new URL(normalized);
  } catch {
    throw new Error("The mobile API URL must be an absolute URL.");
  }
  if (requireHttps && parsed.protocol !== "https:") {
    throw new Error("The mobile API URL must use HTTPS outside local development.");
  }
  if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
    throw new Error("The mobile API URL must use HTTP or HTTPS.");
  }
  if (parsed.username || parsed.password || parsed.search || parsed.hash) {
    throw new Error("The mobile API URL must not contain credentials, a query, or a fragment.");
  }
  return normalized;
}

export function isMobileRole(role: string): role is MobileRole {
  return allowedRoles.has(role);
}

export function normalizeTokenPair(body: TokenPairPayload): MobileSession {
  if (
    typeof body.access_token !== "string" ||
    body.access_token.length < 20 ||
    typeof body.refresh_token !== "string" ||
    body.refresh_token.length < 32 ||
    typeof body.expires_at !== "string" ||
    !Number.isFinite(Date.parse(body.expires_at)) ||
    !body.user ||
    typeof body.user !== "object"
  ) {
    throw new Error(
      typeof body.message === "string" ? body.message : "The authentication response was invalid.",
    );
  }
  const user = body.user as Record<string, unknown>;
  if (
    typeof user.id !== "string" ||
    !user.id ||
    typeof user.name !== "string" ||
    typeof user.email !== "string" ||
    typeof user.role !== "string" ||
    !isMobileRole(user.role) ||
    typeof user.tenant_id !== "string" ||
    !user.tenant_id ||
    !Array.isArray(user.permissions) ||
    !user.permissions.every((permission) => typeof permission === "string")
  ) {
    throw new Error("The authenticated account is not valid for AuraEDU Mobile.");
  }
  return {
    accessToken: body.access_token,
    refreshToken: body.refresh_token,
    expiresAt: body.expires_at,
    user: {
      id: user.id,
      name: user.name,
      email: user.email,
      role: user.role,
      tenant_id: user.tenant_id,
      permissions: user.permissions,
    },
  };
}

export function parseStoredSession(value: string): MobileSession {
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    throw new Error("The stored mobile session is not valid JSON.");
  }
  if (!parsed || typeof parsed !== "object")
    throw new Error("The stored mobile session is invalid.");
  const session = parsed as Record<string, unknown>;
  return normalizeTokenPair({
    access_token: session.accessToken,
    refresh_token: session.refreshToken,
    expires_at: session.expiresAt,
    user: session.user,
  });
}

export function normalizeTenantBranding(
  requestedTenant: string,
  body: TenantBrandingPayload,
): TenantBranding {
  if (!body.tenant_code || !body.name || !body.status) {
    throw new Error(body.message ?? "School could not be resolved.");
  }
  if (body.tenant_code !== requestedTenant) {
    throw new Error("School identity did not match the requested code.");
  }
  if (body.status !== "active") {
    throw new Error("This school's mobile access is not active.");
  }
  const primary = body.branding?.brand?.primary;
  const secondary = body.branding?.brand?.secondary;
  return {
    code: body.tenant_code,
    name: body.name,
    short: body.short?.trim() ? body.short.trim() : body.name,
    status: body.status,
    logoUrl: body.branding?.logo_url ?? undefined,
    primary: primary && hexColor.test(primary) ? primary : tokens.brand.DEFAULT,
    secondary: secondary && hexColor.test(secondary) ? secondary : undefined,
  };
}
import { tokens } from "@auraedu/tokens";
