import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import { resolveTenantFromHost, TENANT_NOT_FOUND_HEADER } from "@/lib/tenant";

const TENANT_COOKIE = "auraedu_tenant_code";
const ACCESS_TOKEN_COOKIE = "auraedu_access_token";
const REFRESH_TOKEN_COOKIE = "auraedu_refresh_token";
const USER_COOKIE = "auraedu_user";

interface TokenPair {
  access_token: string;
  refresh_token: string;
  user?: {
    id: string;
    email: string;
    name?: string;
    role: string;
    tenant_id: string;
  };
}

export default function proxy(request: NextRequest) {
  const host = request.headers.get("host") ?? "";
  const tenantCode = resolveTenantFromHost(host);

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-pathname", request.nextUrl.pathname);
  if (tenantCode) {
    requestHeaders.set(tenantHeaderName, tenantCode);
  }
  if (!tenantCode) {
    requestHeaders.set(TENANT_NOT_FOUND_HEADER, "1");
  }

  const response = NextResponse.next({
    request: {
      headers: requestHeaders,
    },
  });

  response.headers.set("x-resolved-tenant", tenantCode ?? "");

  if (tenantCode) {
    response.cookies.set(TENANT_COOKIE, tenantCode, {
      path: "/",
      sameSite: "lax",
      maxAge: 60 * 60 * 24 * 30,
    });
  } else {
    response.cookies.delete(TENANT_COOKIE);
  }

  const pathname = request.nextUrl.pathname;

  if (!isProtectedPath(pathname)) {
    return response;
  }

  const accessToken = request.cookies.get(ACCESS_TOKEN_COOKIE)?.value;
  if (accessToken && !isTokenExpired(accessToken)) {
    return response;
  }

  const refreshToken = request.cookies.get(REFRESH_TOKEN_COOKIE)?.value;
  if (!refreshToken) {
    return redirectToLogin(request, tenantCode);
  }

  // Synchronous proxy cannot await an external fetch, so perform the refresh
  // asynchronously and let the next response carry the updated cookies. If the
  // refresh fails, redirect to login.
  return refreshTokens(request, tenantCode, refreshToken, response);
}

function isProtectedPath(pathname: string): boolean {
  const publicPrefixes = ["/login", "/api", "/_next", "/favicon", "/sitemap", "/robots", "/not-found"];
  if (publicPrefixes.some((p) => pathname.startsWith(p))) return false;
  const protectedPrefixes = ["/admin", "/teacher", "/student", "/parent", "/superadmin"];
  return protectedPrefixes.some((p) => pathname === p || pathname.startsWith(`${p}/`));
}

function redirectToLogin(request: NextRequest, tenantCode: string | null): NextResponse {
  const url = request.nextUrl.clone();
  url.pathname = "/login";
  if (tenantCode) {
    url.searchParams.set("tenant", tenantCode);
  }
  return NextResponse.redirect(url);
}

async function refreshTokens(
  request: NextRequest,
  tenantCode: string | null,
  refreshToken: string,
  allowResponse: NextResponse,
): Promise<NextResponse> {
  try {
    const res = await fetch(`${publicApiUrl}/api/v1/auth/refresh`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(tenantCode ? { [tenantHeaderName]: tenantCode, "X-Tenant-ID": tenantCode } : {}),
      },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });

    if (!res.ok) {
      return redirectToLogin(request, tenantCode);
    }

    const data = (await res.json()) as TokenPair;
    const accessExp = decodeExpiry(data.access_token) ?? 60 * 15;
    const refreshExp = decodeExpiry(data.refresh_token) ?? 60 * 60 * 24 * 7;

    allowResponse.cookies.set(ACCESS_TOKEN_COOKIE, data.access_token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: accessExp,
    });
    allowResponse.cookies.set(REFRESH_TOKEN_COOKIE, data.refresh_token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: refreshExp,
    });

    if (data.user) {
      allowResponse.cookies.set(USER_COOKIE, JSON.stringify(data.user), {
        httpOnly: false,
        secure: process.env.NODE_ENV === "production",
        sameSite: "lax",
        path: "/",
        maxAge: refreshExp,
      });
    }

    return allowResponse;
  } catch {
    return redirectToLogin(request, tenantCode);
  }
}

function decodeJwtPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const body = parts[1];
    if (!body) return null;
    const base64 = body.replace(/-/g, "+").replace(/_/g, "/");
    const pad = 4 - (base64.length % 4);
    const padded = pad === 4 ? base64 : base64 + "=".repeat(pad);
    return JSON.parse(atob(padded)) as Record<string, unknown>;
  } catch {
    return null;
  }
}

function isTokenExpired(token: string): boolean {
  const payload = decodeJwtPayload(token);
  if (!payload) return true;
  const exp = typeof payload.exp === "number" ? payload.exp : 0;
  if (!exp) return false;
  return Math.floor(Date.now() / 1000) >= exp;
}

function decodeExpiry(token: string): number | undefined {
  const payload = decodeJwtPayload(token);
  if (!payload) return undefined;
  const exp = typeof payload.exp === "number" ? payload.exp : undefined;
  const iat = typeof payload.iat === "number" ? payload.iat : undefined;
  if (exp && iat) return exp - iat;
  if (exp) {
    const remaining = exp - Math.floor(Date.now() / 1000);
    return Math.max(60, remaining);
  }
  return undefined;
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|sitemap.xml|robots.txt|.*\\..*).*)"],
};
