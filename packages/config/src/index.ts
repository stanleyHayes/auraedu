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

export const webEnv = {
  client: webClientSchema.parse(process.env),
  server: webServerSchema.parse(process.env),
};

export const marketingEnv = marketingClientSchema.parse(process.env);

export const publicApiUrl = webEnv.client.NEXT_PUBLIC_API_URL;
export const publicAppUrl = webEnv.client.NEXT_PUBLIC_APP_URL;
export const tenantHeaderName = webEnv.client.NEXT_PUBLIC_TENANT_HEADER;
export const logLevel = webEnv.client.NEXT_PUBLIC_LOG_LEVEL;
export const gatewayInternalUrl = webEnv.server.API_GATEWAY_INTERNAL_URL ?? publicApiUrl;
