"use server";

import { cookies } from "next/headers";
import { createGatewayClient } from "@auraedu/api-client";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import {
  ACCESS_TOKEN_COOKIE,
  REFRESH_TOKEN_COOKIE,
  USER_COOKIE,
  roleHomePath,
  type UserSession,
} from "@/lib/auth";

const TENANT_COOKIE = "auraedu_tenant_code";

export interface LoginResult {
  success?: boolean;
  error?: string;
  redirectTo?: string;
}

interface LoginResponse {
  access_token: string;
  refresh_token: string;
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
  const tenantCode = jar.get(TENANT_COOKIE)?.value ?? "";

  const email = String((formData.get("email") as string | null) ?? "").trim();
  const password = String((formData.get("password") as string | null) ?? "");

  if (!email || !password) {
    return { error: "Email and password are required." };
  }

  const client = createGatewayClient({
    baseUrl: publicApiUrl,
    tenantHeader: tenantHeaderName,
    getToken: () => undefined,
    getTenantCode: () => tenantCode,
  });

  try {
    const data = await client.post<LoginResponse>("/api/v1/auth/login", {
      email,
      password,
    });

    const accessToken = data.access_token;
    const refreshToken = data.refresh_token;
    const user = data.user;

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

    return { success: true, redirectTo: roleHomePath(user.role) };
  } catch (e) {
    const err = e as { status?: number; code?: string; message?: string };
    return { error: safeErrorMessage(err.status ?? 500) };
  }
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
