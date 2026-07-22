import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";
import { getCurrentTenantCode, getCurrentToken } from "@/lib/api";

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export async function GET(
  _request: Request,
  context: { params: Promise<{ report_card_id: string }> },
) {
  const { report_card_id: reportCardID } = await context.params;
  if (!uuidPattern.test(reportCardID)) {
    return Response.json({ error: "not_found", message: "Report card not found" }, { status: 404 });
  }

  const [token, tenantCode] = await Promise.all([getCurrentToken(), getCurrentTenantCode()]);
  if (!token || !tenantCode) {
    return Response.json(
      { error: "unauthorized", message: "Authentication required" },
      { status: 401 },
    );
  }

  let upstream: Response;
  try {
    upstream = await fetch(
      `${gatewayInternalUrl.replace(/\/$/, "")}/api/v1/report-cards/${encodeURIComponent(reportCardID)}/download`,
      {
        headers: {
          Authorization: `Bearer ${token}`,
          [tenantHeaderName]: tenantCode,
        },
        cache: "no-store",
        redirect: "error",
      },
    );
  } catch {
    return Response.json(
      { error: "download_unavailable", message: "Report PDF is unavailable" },
      { status: 502 },
    );
  }
  if (!upstream.ok || !upstream.body) {
    const status =
      upstream.status === 401 || upstream.status === 403 || upstream.status === 404
        ? upstream.status
        : 502;
    return Response.json(
      {
        error: status === 404 ? "not_found" : "download_unavailable",
        message: "Report PDF is unavailable",
      },
      { status },
    );
  }

  const headers = new Headers({
    "Content-Type": "application/pdf",
    "Content-Disposition": `attachment; filename="report-card-${reportCardID}.pdf"`,
    "Cache-Control": "private, no-store",
    "X-Content-Type-Options": "nosniff",
  });
  const length = upstream.headers.get("content-length");
  if (length) headers.set("Content-Length", length);
  return new Response(upstream.body, { status: 200, headers });
}
