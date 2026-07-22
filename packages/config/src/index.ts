import { z } from "zod";

const webClientSchema = z.object({
  NEXT_PUBLIC_API_URL: z.string().url().default("http://localhost:8080"),
  NEXT_PUBLIC_APP_URL: z.string().url().default("http://localhost:3000"),
  NEXT_PUBLIC_TENANT_HEADER: z.string().min(1).default("x-tenant-code"),
  NEXT_PUBLIC_LOG_LEVEL: z.enum(["debug", "info", "warn", "error"]).default("info"),
});

const webServerSchema = z.object({
  API_GATEWAY_INTERNAL_URL: z.string().url().optional(),
});

const marketingClientSchema = z.object({
  NEXT_PUBLIC_API_URL: z.string().url().default("http://localhost:8080"),
  NEXT_PUBLIC_MARKETING_URL: z.string().url().default("http://localhost:3001"),
  NEXT_PUBLIC_LOG_LEVEL: z.enum(["debug", "info", "warn", "error"]).default("info"),
});

function assertSafePublicUrls(
  environment: Record<string, string | undefined>,
  values: Record<string, string>,
) {
  const production = environment.ENVIRONMENT?.trim().toLowerCase() === "production";
  for (const [key, value] of Object.entries(values)) {
    const url = new URL(value);
    if (url.username || url.password || url.search || url.hash) {
      throw new Error(`${key} must not contain credentials, query parameters, or a fragment.`);
    }
    if (
      production &&
      (url.protocol !== "https:" || url.hostname === "localhost" || url.hostname === "127.0.0.1")
    ) {
      throw new Error(`${key} must use a non-loopback HTTPS origin in production.`);
    }
  }
}

export function parseWebClientEnv(env: Record<string, string | undefined>) {
  const parsed = webClientSchema.parse(env);
  assertSafePublicUrls(env, {
    NEXT_PUBLIC_API_URL: parsed.NEXT_PUBLIC_API_URL,
    NEXT_PUBLIC_APP_URL: parsed.NEXT_PUBLIC_APP_URL,
  });
  return parsed;
}

export function parseWebServerEnv(env: Record<string, string | undefined>) {
  return webServerSchema.parse(env);
}

export function parseMarketingClientEnv(env: Record<string, string | undefined>) {
  const parsed = marketingClientSchema.parse(env);
  assertSafePublicUrls(env, {
    NEXT_PUBLIC_API_URL: parsed.NEXT_PUBLIC_API_URL,
    NEXT_PUBLIC_MARKETING_URL: parsed.NEXT_PUBLIC_MARKETING_URL,
  });
  return parsed;
}

// Next.js replaces direct NEXT_PUBLIC_* property reads at build time. Passing
// the opaque `process.env` object to Zod leaves client bundles with the schema
// defaults instead, which can silently point a deployed or remapped frontend
// at localhost:8080. Keep these reads explicit so the validated values are the
// values embedded in browser code.
const webClientRuntimeEnv = {
  NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL,
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL,
  NEXT_PUBLIC_TENANT_HEADER: process.env.NEXT_PUBLIC_TENANT_HEADER,
  NEXT_PUBLIC_LOG_LEVEL: process.env.NEXT_PUBLIC_LOG_LEVEL,
  ENVIRONMENT: process.env.ENVIRONMENT,
};

const webServerRuntimeEnv = {
  API_GATEWAY_INTERNAL_URL: process.env.API_GATEWAY_INTERNAL_URL,
};

export const webEnv = {
  client: parseWebClientEnv(webClientRuntimeEnv),
  server: parseWebServerEnv(webServerRuntimeEnv),
};

export function getMarketingEnv() {
  return parseMarketingClientEnv({
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL,
    NEXT_PUBLIC_MARKETING_URL: process.env.NEXT_PUBLIC_MARKETING_URL,
    NEXT_PUBLIC_LOG_LEVEL: process.env.NEXT_PUBLIC_LOG_LEVEL,
    ENVIRONMENT: process.env.ENVIRONMENT,
  });
}

export const publicApiUrl = webEnv.client.NEXT_PUBLIC_API_URL;
export const publicAppUrl = webEnv.client.NEXT_PUBLIC_APP_URL;
export const tenantHeaderName = webEnv.client.NEXT_PUBLIC_TENANT_HEADER;
export const logLevel = webEnv.client.NEXT_PUBLIC_LOG_LEVEL;
export const gatewayInternalUrl = webEnv.server.API_GATEWAY_INTERNAL_URL ?? publicApiUrl;
