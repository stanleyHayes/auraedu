// Liveness probe for Render (render.yaml healthCheckPath: /api/health).
export const dynamic = "force-static";

export function GET() {
  return Response.json({ status: "ok", service: "marketing" });
}
