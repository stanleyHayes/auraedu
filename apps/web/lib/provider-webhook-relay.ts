import "server-only";

const maxCallbackBytes = 128 * 1024;

interface RelayOptions {
  callbackPath: string;
  contentType: string;
  forwardedHeaders: string[];
  preserveQuery?: boolean;
}

function gatewayOrigin(): URL | null {
  const configured = process.env.AURAEDU_API_URL ?? process.env.NEXT_PUBLIC_API_URL;
  if (!configured) return null;
  try {
    const origin = new URL(configured);
    const production = process.env.NODE_ENV === "production";
    const host = origin.hostname.toLowerCase();
    if (
      !["http:", "https:"].includes(origin.protocol) ||
      (production &&
        (origin.protocol !== "https:" ||
          origin.port !== "" ||
          ["localhost", "127.0.0.1", "::1"].includes(host) ||
          host.endsWith(".example"))) ||
      origin.username ||
      origin.password ||
      origin.search ||
      origin.hash ||
      origin.pathname !== "/"
    ) {
      return null;
    }
    return origin;
  } catch {
    return null;
  }
}

async function readBoundedBody(request: Request): Promise<Uint8Array<ArrayBuffer> | null> {
  if (!request.body) return new Uint8Array(0);
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > maxCallbackBytes) {
      await reader.cancel().catch(() => undefined);
      return null;
    }
    chunks.push(value);
  }
  const body = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}

// Relays only the exact provider-signature headers and never holds provider
// credentials. The backend remains the sole signature verifier and state owner.
export async function relayProviderWebhook(request: Request, options: RelayOptions) {
  const origin = gatewayOrigin();
  if (!origin) {
    return Response.json({ error: "delivery_feedback_unavailable" }, { status: 503 });
  }
  const contentType = request.headers.get("content-type") ?? "";
  const rawLength = request.headers.get("content-length") ?? "";
  if (
    !contentType.toLowerCase().startsWith(options.contentType) ||
    (rawLength !== "" && (!/^\d+$/.test(rawLength) || Number(rawLength) > maxCallbackBytes))
  ) {
    return Response.json({ error: "invalid_provider_event" }, { status: 422 });
  }
  const body = await readBoundedBody(request);
  if (!body) {
    return Response.json({ error: "invalid_provider_event" }, { status: 422 });
  }
  const target = new URL(options.callbackPath, origin);
  if (options.preserveQuery) target.search = new URL(request.url).search;
  const headers = new Headers({ "content-type": contentType });
  for (const name of options.forwardedHeaders) {
    headers.set(name, request.headers.get(name) ?? "");
  }
  try {
    const upstream = await fetch(target, {
      method: "POST",
      headers,
      body,
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
    });
    return new Response(null, { status: upstream.status });
  } catch {
    return Response.json({ error: "delivery_feedback_unavailable" }, { status: 503 });
  }
}
