#!/usr/bin/env node

import { randomBytes } from "node:crypto";
import { chmodSync, existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const ENV_PATH = join(ROOT, ".env");
const PRIVATE_DIR = join(ROOT, ".auraedu-local");
const DATABASE_URLS_PATH = join(PRIVATE_DIR, "migration-database-urls.json");
const MOBILE_ENV_PATH = join(ROOT, "apps/mobile/.env.local");

const DATABASES = {
  "academic-service": "academic",
  "admissions-service": "admissions",
  "ai-orchestrator-service": "assistant",
  "ai-prediction-service": "ai",
  "ai-recommendation-service": "ai",
  "analytics-service": "analytics",
  "assessment-service": "assessment",
  "attendance-service": "attendance",
  "audit-service": "audit",
  "billing-service": "billing",
  "campaign-service": "campaign",
  "content-service": "content",
  "career-guidance-service": "ai",
  "cbt-service": "cbt",
  "crm-service": "crm",
  "fees-service": "fees",
  "file-service": "file",
  "identity-service": "identity",
  "knowledge-service": "knowledge",
  "market-intelligence-service": "intelligence",
  "notification-service": "notification",
  "payment-service": "payment",
  "report-service": "report",
  "staff-service": "staff",
  "student-service": "student",
  "tenant-service": "tenant",
  "website-service": "website",
};

function parseKeys(content) {
  const keys = new Set();
  for (const line of content.split(/\r?\n/)) {
    const match = /^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=/.exec(line);
    if (match) keys.add(match[1]);
  }
  return keys;
}

function parseRawValues(content) {
  const values = new Map();
  for (const line of content.split(/\r?\n/)) {
    const match = /^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=(.*)$/.exec(line);
    if (match) values.set(match[1], match[2].trim());
  }
  return values;
}

function unquote(raw) {
  if (raw.length >= 2) {
    const first = raw[0];
    const last = raw[raw.length - 1];
    if ((first === '"' && last === '"') || (first === "'" && last === "'")) {
      return raw.slice(1, -1);
    }
  }
  return raw;
}

function promoteResendCredential(path) {
  if (!existsSync(path)) return false;
  const original = readFileSync(path, "utf8");
  const values = parseRawValues(original);
  if (unquote(values.get("NOTIFICATION_PROVIDER") ?? "").toLowerCase() !== "resend") return false;
  if (unquote(values.get("RESEND_API_KEY") ?? "") !== "") return false;

  const smtpPassword = values.get("SMTP_PASSWORD") ?? "";
  if (unquote(smtpPassword) === "") return false;

  let replaced = false;
  const updated = original
    .split(/\r?\n/)
    .map((line) => {
      if (!/^\s*(?:export\s+)?RESEND_API_KEY\s*=/.test(line)) return line;
      replaced = true;
      return `RESEND_API_KEY=${smtpPassword}`;
    })
    .join("\n");
  if (!replaced) return false;

  const temporary = `${path}.${process.pid}.tmp`;
  writeFileSync(temporary, updated, { mode: 0o600 });
  renameSync(temporary, path);
  chmodSync(path, 0o600);
  return true;
}

function appendMissing(path, entries, heading) {
  const original = existsSync(path) ? readFileSync(path, "utf8") : "";
  const keys = parseKeys(original);
  const missing = Object.entries(entries).filter(([key]) => !keys.has(key));
  if (missing.length === 0) {
    chmodSync(path, 0o600);
    return [];
  }
  const separator = original.length > 0 && !original.endsWith("\n") ? "\n" : "";
  const block = `${separator}${original.length > 0 ? "\n" : ""}# ${heading}\n${missing
    .map(([key, value]) => `${key}=${value}`)
    .join("\n")}\n`;
  const temporary = `${path}.${process.pid}.tmp`;
  writeFileSync(temporary, original + block, { mode: 0o600 });
  renameSync(temporary, path);
  chmodSync(path, 0o600);
  return missing.map(([key]) => key);
}

function secret(bytes = 32) {
  return randomBytes(bytes).toString("base64url");
}

mkdirSync(PRIVATE_DIR, { recursive: true, mode: 0o700 });
chmodSync(PRIVATE_DIR, 0o700);

const rootCreated = appendMissing(
  ENV_PATH,
  {
    JWT_SIGNING_KEY: secret(48),
    MFA_ENCRYPTION_KEY: secret(48),
    INTERNAL_SERVICE_TOKEN: secret(48),
    NOTIFICATION_UNSUBSCRIBE_SIGNING_KEY: secret(48),
    METRICS_BEARER_TOKEN: secret(32),
    GRAFANA_ADMIN_USER: "admin",
    GRAFANA_ADMIN_PASSWORD: secret(32),
    PROMETHEUS_PORT: "19090",
    GRAFANA_PORT: "13000",
    GATEWAY_HOST_PORT: "18080",
    STUDENT_HOST_PORT: "28090",
    WEB_HOST_PORT: "13100",
    MARKETING_HOST_PORT: "13101",
    AURA_MIGRATION_DATABASE_URLS_FILE: DATABASE_URLS_PATH,
  },
  "Generated local-only AuraEDU values. Never commit this file.",
);

const externalCreated = appendMissing(
  ENV_PATH,
  {
    CLOUDINARY_URL: "",
    CLOUDINARY_RESOURCE_TYPE: "raw",
    PAYMENTS_PROVIDER: "mock",
    PAYSTACK_SECRET_KEY: "",
    PAYSTACK_WEBHOOK_SECRET: "",
    NOTIFICATION_PROVIDER: "mock",
    SMTP_HOST: "",
    SMTP_USERNAME: "",
    SMTP_PASSWORD: "",
    SMTP_PORT: "587",
    SMTP_FROM_EMAIL: "",
    SMTP_FROM_NAME: "AuraEDU",
    SMTP_ALLOW_INSECURE: "false",
    RESEND_API_KEY: "",
    RESEND_WEBHOOK_SECRET: "",
    RESEND_FROM_EMAIL: "onboarding@resend.dev",
    RESEND_FROM_NAME: "AuraEDU",
    RESEND_API_BASE: "https://api.resend.com",
    EXPO_ACCESS_TOKEN: "",
    EXPO_PUSH_URL: "https://exp.host/--/api/v2/push/send",
    TWILIO_ACCOUNT_SID: "",
    TWILIO_AUTH_TOKEN: "",
    TWILIO_SMS_FROM: "",
    TWILIO_MESSAGING_SERVICE_SID: "",
    TWILIO_WHATSAPP_FROM: "",
    TWILIO_STATUS_CALLBACK_URL: "https://auraedugh.vercel.app/api/v1/webhooks/twilio",
    OPENAI_API_KEY: "",
    OPENAI_BASE_URL: "https://api.openai.com",
    CONTENT_AI_MODEL: "gpt-5.6-sol",
    LOG_LEVEL: "info",
    OTEL_TRACES_SAMPLER_ARG: "1.0",
    OTEL_EXPORTER_OTLP_ENDPOINT: "",
    GATEWAY_CORS_ORIGINS: "",
    NEXT_PUBLIC_API_URL: "",
    NEXT_PUBLIC_APP_URL: "",
    AURAEDU_API_URL: "",
    PUBLIC_APP_URL: "",
    DR_BACKUP_S3_ENDPOINT: "",
    DR_BACKUP_S3_REGION: "eu-central-1",
    DR_BACKUP_S3_BUCKET: "",
    DR_BACKUP_S3_PREFIX: "auraedu",
    DR_BACKUP_S3_ACCESS_KEY_ID: "",
    DR_BACKUP_S3_SECRET_ACCESS_KEY: "",
    DR_BACKUP_HEARTBEAT_URL: "",
    DR_BACKUP_HEARTBEAT_TOKEN: "",
    DR_BACKUP_ALERT_URL: "",
    DR_BACKUP_ALERT_TOKEN: "",
    DR_POSTGRES_BACKUP_HEARTBEAT_URL: "",
    DR_POSTGRES_BACKUP_HEARTBEAT_TOKEN: "",
    DR_POSTGRES_BACKUP_ALERT_URL: "",
    DR_POSTGRES_BACKUP_ALERT_TOKEN: "",
    AURA_PERF_BASE_URL: "",
    AURA_PERF_TENANTS_JSON: "",
    AURA_PERF_DURATION: "5m",
    AURA_PERF_CONCURRENCY: "20",
    AURA_PERF_RUN_ID: "",
    AURA_PERF_GIT_SHA: "",
    AURA_ISOLATION_BASE_URL: "",
    AURA_ISOLATION_SCHOOLS_JSON: "",
    AURA_ISOLATION_RUN_ID: "",
    AURA_ISOLATION_GIT_SHA: "",
    AURA_PROVIDER_BASE_URL: "",
    AURA_PROVIDER_RUN_ID: "",
    AURA_PROVIDER_GIT_SHA: "",
    AURA_PROVIDER_TENANT: "",
    AURA_PROVIDER_TOKEN: "",
    AURA_PROVIDER_RECIPIENT_ID: "",
    AURA_PROVIDER_EMAIL: "",
    AURA_TWILIO_BASE_URL: "",
    AURA_TWILIO_RUN_ID: "",
    AURA_TWILIO_GIT_SHA: "",
    AURA_TWILIO_TENANT: "",
    AURA_TWILIO_TOKEN: "",
    AURA_TWILIO_RECIPIENT_ID: "",
    AURA_TWILIO_SMS_NUMBER: "",
    AURA_TWILIO_WHATSAPP_NUMBER: "",
    AURA_VERCEL_RUN_ID: "",
    AURA_VERCEL_GIT_SHA: "",
    AURA_VERCEL_TOKEN: "",
    AURA_VERCEL_TEAM_ID: "",
    AURA_VERCEL_WEB_PROJECT: "",
    AURA_VERCEL_MARKETING_PROJECT: "",
    AURA_VERCEL_WEB_URL: "https://auraedugh.vercel.app",
    AURA_VERCEL_MARKETING_URL: "",
    AURA_VERCEL_GATEWAY_URL: "",
  },
  "Values you must supply for providers, production URLs, DR, and release evidence.",
);
const resendCredentialPromoted = promoteResendCredential(ENV_PATH);

const databaseUrls = Object.fromEntries(
  Object.entries(DATABASES).map(([service, database]) => [
    service,
    `postgres://auraedu:auraedu@127.0.0.1:5432/${database}?sslmode=disable`,
  ]),
);
writeFileSync(DATABASE_URLS_PATH, `${JSON.stringify(databaseUrls, null, 2)}\n`, { mode: 0o600 });
chmodSync(DATABASE_URLS_PATH, 0o600);

const mobileCreated = appendMissing(
  MOBILE_ENV_PATH,
  {
    APP_ENV: "development",
    EXPO_PUBLIC_API_URL: "http://127.0.0.1:18080",
    EAS_PROJECT_ID: "",
  },
  "Generated local-only mobile development values. Never commit this file.",
);

console.log(`Local configuration ready (secrets were not printed).`);
console.log(`- .env: ${rootCreated.length ? `created ${rootCreated.join(", ")}` : "preserved"}`);
console.log(
  `- external placeholders: ${externalCreated.length ? `created ${externalCreated.join(", ")}` : "preserved"}`,
);
console.log(
  `- Resend API credential: ${resendCredentialPromoted ? "moved from the SMTP password slot without printing it" : "preserved"}`,
);
console.log(
  `- ${DATABASE_URLS_PATH}: wrote ${Object.keys(databaseUrls).length} service URLs (mode 600)`,
);
console.log(
  `- apps/mobile/.env.local: ${mobileCreated.length ? `created ${mobileCreated.join(", ")}` : "preserved"}`,
);
console.log(
  "Provider-owned Cloudinary, Resend/email, Expo/EAS, store, domain, and deployment credentials were not fabricated.",
);
