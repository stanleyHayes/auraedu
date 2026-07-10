// Server-side proxy: browser → this route → Tenant Service (EP-05). Keeps the
// internal service URL off the client and injects the actor the service requires.
//
// TEMPORARY until the API Gateway + Identity land. Because there is no real auth yet,
// this route forges a session actor — so it is deliberately locked down:
//   • it only serves the KNOWN PREVIEW TENANTS (allowlist), never arbitrary codes, which
//     both bounds the forged-actor blast radius and prevents URL/argument injection;
//   • the forged actor carries NO permissions (this route only reads; it never manages
//     features), and its tenant scope matches the requested demo tenant.
// In production the browser calls the gateway, which verifies the JWT and injects the
// real X-Actor-* headers itself (see platform/auth).
import { PREVIEW_TENANT_CODES } from "@/lib/tenant";

export const dynamic = "force-dynamic";

const TENANT_SERVICE = process.env.TENANT_SERVICE_URL ?? "http://localhost:8082";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ code: string }> },
) {
  const { code } = await params;

  // Guardrail: demo shim serves only the known preview tenants.
  if (!PREVIEW_TENANT_CODES.includes(code)) {
    return Response.json({ error: "unknown tenant" }, { status: 404 });
  }

  // Minimal, unprivileged actor scoped to this demo tenant (no permissions).
  const actor: HeadersInit = {
    "X-Actor-User": "web-preview",
    "X-Actor-Tenant": code,
  };
  const seg = encodeURIComponent(code);

  try {
    const [tenantRes, featuresRes] = await Promise.all([
      fetch(`${TENANT_SERVICE}/api/v1/tenants/${seg}`, { headers: actor, cache: "no-store" }),
      fetch(`${TENANT_SERVICE}/api/v1/features?tenant=${seg}`, { headers: actor, cache: "no-store" }),
    ]);

    if (!tenantRes.ok) {
      return Response.json({ error: "tenant not found" }, { status: tenantRes.status });
    }

    const tenant = await tenantRes.json();
    const features = featuresRes.ok ? ((await featuresRes.json()) as { features: unknown[] }).features : [];
    return Response.json({ tenant, features });
  } catch {
    return Response.json({ error: "tenant service unavailable" }, { status: 502 });
  }
}
