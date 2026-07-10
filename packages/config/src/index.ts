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

function parseEnv<T>(schema: z.ZodSchema<T>, label: string): T {
  const result = schema.safeParse(process.env);
  if (!result.success) {
    const issues = result.error.issues.map((i) => `${i.path.join(".")}: ${i.message}`).join("; ");
    throw new Error(`Invalid ${label} environment: ${issues}`);
  }
  return result.data;
}

export const webEnv = {
  client: parseEnv(webClientSchema, "web client"),
  server: parseEnv(webServerSchema, "web server"),
};

export const marketingEnv = parseEnv(marketingClientSchema, "marketing");

export const publicApiUrl = webEnv.client.NEXT_PUBLIC_API_URL;
export const publicAppUrl = webEnv.client.NEXT_PUBLIC_APP_URL;
export const tenantHeaderName = webEnv.client.NEXT_PUBLIC_TENANT_HEADER;
export const logLevel = webEnv.client.NEXT_PUBLIC_LOG_LEVEL;
export const gatewayInternalUrl = webEnv.server.API_GATEWAY_INTERNAL_URL ?? publicApiUrl;
