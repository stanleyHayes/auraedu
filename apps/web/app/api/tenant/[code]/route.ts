// Server-side proxy: browser → this route → Tenant Service (EP-05). Keeps the
// internal service URL off the client and injects the actor the service requires.
//
// TEMPORARY until the API Gateway + Identity land: we forge a session actor for the
// requested tenant so the (authorized) Tenant Service returns its record + features.
// In production the browser calls the gateway, which verifies the JWT and injects the
// X-Actor-* headers itself (see platform/auth + apps/web/lib/tenant.ts).
export const dynamic = "force-dynamic";

const TENANT_SERVICE = process.env.TENANT_SERVICE_URL ?? "http://localhost:8082";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ code: string }> },
) {
  const { code } = await params;
  const actor: HeadersInit = {
    "X-Actor-User": "web-preview",
    "X-Actor-Tenant": code,
    "X-Actor-Permissions": "features.manage",
  };

  try {
    const [tenantRes, featuresRes] = await Promise.all([
      fetch(`${TENANT_SERVICE}/api/v1/tenants/${code}`, { headers: actor, cache: "no-store" }),
      fetch(`${TENANT_SERVICE}/api/v1/features?tenant=${code}`, { headers: actor, cache: "no-store" }),
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
