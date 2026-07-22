import type { ConfigContext, ExpoConfig } from "expo/config";
import { URL } from "node:url";
import app from "./app.json";

type AppEnvironment = "development" | "staging" | "production";
const environmentVariables =
  (
    globalThis as unknown as {
      process?: { env?: Record<string, string | undefined> };
    }
  ).process?.env ?? {};

function environment(): AppEnvironment {
  const value = environmentVariables.APP_ENV ?? "development";
  if (value === "development" || value === "staging" || value === "production") return value;
  throw new Error(`APP_ENV must be development, staging, or production; received ${value}.`);
}

function apiUrl(appEnvironment: AppEnvironment): string {
  const configured = environmentVariables.EXPO_PUBLIC_API_URL?.trim();
  const fallback =
    appEnvironment === "production"
      ? "https://api.auraedu.com"
      : appEnvironment === "development"
        ? "http://127.0.0.1:8080"
        : "";
  const value = configured && configured.length > 0 ? configured : fallback;
  if (!value) {
    throw new Error(`EXPO_PUBLIC_API_URL is required for ${appEnvironment} mobile builds.`);
  }
  const parsed = new URL(value);
  if (appEnvironment !== "development" && parsed.protocol !== "https:") {
    throw new Error(`${appEnvironment} mobile builds require an HTTPS API URL.`);
  }
  if (parsed.username || parsed.password || parsed.search || parsed.hash) {
    throw new Error(
      "EXPO_PUBLIC_API_URL must not include credentials, query parameters, or a fragment.",
    );
  }
  return value.replace(/\/+$/, "");
}

export default ({ config }: ConfigContext): ExpoConfig => {
  const appEnvironment = environment();
  const projectId = environmentVariables.EAS_PROJECT_ID?.trim();
  if (environmentVariables.EAS_BUILD === "true" && appEnvironment !== "development" && !projectId) {
    throw new Error(
      "EAS_PROJECT_ID is required for preview and production builds so push delivery cannot be silently disabled.",
    );
  }

  const base = app.expo as ExpoConfig;
  const rawExtra: unknown = base.extra;
  const baseExtra =
    rawExtra && typeof rawExtra === "object" ? (rawExtra as Record<string, unknown>) : {};
  return {
    ...config,
    ...base,
    extra: {
      ...baseExtra,
      apiUrl: apiUrl(appEnvironment),
      appEnvironment,
      easProjectId: projectId,
      eas: projectId ? { projectId } : undefined,
    },
    updates: projectId
      ? {
          url: `https://u.expo.dev/${projectId}`,
          enabled: appEnvironment !== "development",
          checkAutomatically: "ON_LOAD",
          fallbackToCacheTimeout: 0,
        }
      : { enabled: false },
  };
};
