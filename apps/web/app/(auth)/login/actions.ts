"use server";

import { cookies } from "next/headers";
import { createGatewayClient } from "@auraedu/api-client";
import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";
import {
  ACCESS_TOKEN_COOKIE,
  REFRESH_TOKEN_COOKIE,
  USER_COOKIE,
  roleHomePath,
  type UserSession,
} from "@/lib/auth";
import { safePostLoginPath } from "@/lib/safe-redirect";
import { canonicalTenantCode } from "@/lib/tenant";

const TENANT_COOKIE = "auraedu_tenant_code";

export interface LoginResult {
  success?: boolean;
  error?: string;
  redirectTo?: string;
  mfa?: {
    status: "mfa_required" | "mfa_setup_required";
    challengeToken: string;
    setupSecret?: string;
    tenantCode: string;
    nextPath: string;
  };
}

interface LoginResponse {
  status?: "mfa_required" | "mfa_setup_required";
  challenge_token?: string;
  secret?: string;
  otpauth_uri?: string;
  access_token?: string;
  refresh_token?: string;
  token_type?: string;
  expires_at?: string;
  user: {
    id: string;
    email: string;
    name?: string;
    role: string;
    tenant_id: string;
  };
}

function safeErrorMessage(status: number): string {
  if (status === 401 || status === 404) return "Invalid email or password.";
  if (status === 403) return "You do not have access to this school.";
  if (status === 422) return "Please check your email and password.";
  return "Unable to sign in. Please try again.";
}

export async function loginAction(_prev: LoginResult, formData: FormData): Promise<LoginResult> {
  const jar = await cookies();
  const submittedTenant = canonicalTenantCode(
    String((formData.get("tenant") as string | null) ?? ""),
  );
  const tenantCode = submittedTenant || canonicalTenantCode(jar.get(TENANT_COOKIE)?.value);

  const email = String((formData.get("email") as string | null) ?? "").trim();
  const password = String((formData.get("password") as string | null) ?? "");
  const nextPath = String((formData.get("next") as string | null) ?? "");

  if (!email || !password) {
    return { error: "Email and password are required." };
  }
  if (!tenantCode) {
    return { error: "Enter your school workspace code or open your school's portal link." };
  }

  jar.set(TENANT_COOKIE, tenantCode, {
    httpOnly: false,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 30,
  });

  const client = createGatewayClient({
    baseUrl: gatewayInternalUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => undefined,
    getTenantCode: () => tenantCode,
  });

  try {
    const data = await client.post<LoginResponse>("/api/v1/auth/login", {
      email,
      password,
    });

    if (data.status === "mfa_required" || data.status === "mfa_setup_required") {
      if (!data.challenge_token || (data.status === "mfa_setup_required" && !data.secret)) {
        return { error: "Unable to start secure sign-in. Please try again." };
      }
      return {
        mfa: {
          status: data.status,
          challengeToken: data.challenge_token,
          setupSecret: data.secret,
          tenantCode,
          nextPath,
        },
      };
    }

    const accessToken = data.access_token;
    const refreshToken = data.refresh_token;
    const user = data.user;

    if (!accessToken || !refreshToken) {
      return { error: "Unable to complete sign-in. Please try again." };
    }

    if (user.role !== "platform_super_admin" && user.tenant_id !== tenantCode) {
      return { error: "Invalid email or password." };
    }

    setSessionCookies(jar, accessToken, refreshToken, user);

    return {
      success: true,
      redirectTo: nextPath ? safePostLoginPath(nextPath, user.role) : roleHomePath(user.role),
    };
  } catch (e) {
    const err = e as { status?: number; code?: string; message?: string };
    return { error: safeErrorMessage(err.status ?? 500) };
  }
}

export async function verifyMFAAction(
  _prev: LoginResult,
  formData: FormData,
): Promise<LoginResult> {
  const challengeToken = formString(formData, "challenge_token");
  const setupSecret = formString(formData, "setup_secret");
  const code = formString(formData, "code").replace(/\s/g, "");
  const tenantCode = canonicalTenantCode(formString(formData, "tenant"));
  const nextPath = formString(formData, "next");

  if (!challengeToken || !tenantCode || !/^\d{6}$/.test(code)) {
    return { error: "Enter the six-digit code from your authenticator app." };
  }

  const client = createGatewayClient({
    baseUrl: gatewayInternalUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => undefined,
    getTenantCode: () => tenantCode,
  });

  try {
    const data = await client.post<LoginResponse>("/api/v1/auth/mfa/verify", {
      challenge_token: challengeToken,
      code,
      ...(setupSecret ? { setup_secret: setupSecret } : {}),
    });
    if (!data.access_token || !data.refresh_token) {
      return { error: "Unable to complete secure sign-in. Please try again." };
    }
    if (data.user.role !== "platform_super_admin" && data.user.tenant_id !== tenantCode) {
      return { error: "This code is not valid for this school workspace." };
    }
    setSessionCookies(await cookies(), data.access_token, data.refresh_token, data.user);
    return {
      success: true,
      redirectTo: nextPath
        ? safePostLoginPath(nextPath, data.user.role)
        : roleHomePath(data.user.role),
    };
  } catch {
    return { error: "That code is invalid, expired, or has already been used." };
  }
}

function formString(formData: FormData, key: string): string {
  const value = formData.get(key);
  return typeof value === "string" ? value : "";
}

function setSessionCookies(
  jar: Awaited<ReturnType<typeof cookies>>,
  accessToken: string,
  refreshToken: string,
  user: LoginResponse["user"],
) {
  const accessExp = decodeExpiry(accessToken) ?? 60 * 15;
  const refreshExp = decodeExpiry(refreshToken) ?? 60 * 60 * 24 * 7;

  jar.set(ACCESS_TOKEN_COOKIE, accessToken, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: accessExp,
  });
  jar.set(REFRESH_TOKEN_COOKIE, refreshToken, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: refreshExp,
  });

  const userPayload: Omit<UserSession, "permissions" | "exp"> = {
    sub: user.id,
    user_id: user.id,
    tenant_id: user.tenant_id,
    role: user.role,
    email: user.email,
    name: user.name ?? user.email,
  };
  jar.set(USER_COOKIE, JSON.stringify(userPayload), {
    httpOnly: false,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: refreshExp,
  });
}

function decodeExpiry(token: string): number | undefined {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return undefined;
    const body = parts[1];
    if (!body) return undefined;
    const base64 = body.replace(/-/g, "+").replace(/_/g, "/");
    const pad = 4 - (base64.length % 4);
    const padded = pad === 4 ? base64 : base64 + "=".repeat(pad);
    const payload = JSON.parse(atob(padded)) as { exp?: number; iat?: number };
    if (typeof payload.exp === "number" && typeof payload.iat === "number") {
      return payload.exp - payload.iat;
    }
    if (typeof payload.exp === "number") {
      const now = Math.floor(Date.now() / 1000);
      return Math.max(60, payload.exp - now);
    }
  } catch {
    /* ignore */
  }
  return undefined;
}
