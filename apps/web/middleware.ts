import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { tenantHeaderName } from "@auraedu/config";
import { resolveTenantFromHost } from "@/lib/tenant";

export function middleware(request: NextRequest) {
  const host = request.headers.get("host") ?? "";
  const tenantCode = resolveTenantFromHost(host);

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set(tenantHeaderName, tenantCode);

  const response = NextResponse.next({
    request: {
      headers: requestHeaders,
    },
  });

  response.headers.set("x-resolved-tenant", tenantCode);
  return response;
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|sitemap.xml|robots.txt|.*\\..*).*)",
  ],
};
