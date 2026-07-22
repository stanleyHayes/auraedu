#!/usr/bin/env node
/* global console, process, URL */

const app = process.argv[2];
if (app !== "web" && app !== "marketing") {
  console.error("Usage: validate-environment.mjs <web|marketing>");
  process.exit(2);
}

const production =
  process.env.VERCEL_ENV === "production" ||
  process.env.ENVIRONMENT?.trim().toLowerCase() === "production";
const required = ["NEXT_PUBLIC_API_URL", "NEXT_PUBLIC_APP_URL"];
const failures = [];

function validateOrigin(key, requiredInProduction = true) {
  const raw = process.env[key]?.trim() ?? "";
  if (!raw) {
    if (production && requiredInProduction) failures.push(`${key} is required in production`);
    return;
  }
  let url;
  try {
    url = new URL(raw);
  } catch {
    failures.push(`${key} must be an absolute URL`);
    return;
  }
  if (url.username || url.password || url.search || url.hash) {
    failures.push(`${key} must be a clean origin without credentials, query, or fragment`);
  }
  if (url.pathname !== "/") failures.push(`${key} must not contain a path`);
  if (
    production &&
    (url.protocol !== "https:" || url.hostname === "localhost" || url.hostname === "127.0.0.1")
  ) {
    failures.push(`${key} must be a non-loopback HTTPS origin in production`);
  }
}

required.forEach((key) => validateOrigin(key));
if (app === "marketing") {
  validateOrigin("AURAEDU_API_URL", false);
  validateOrigin("API_GATEWAY_URL", false);
}

if (failures.length > 0) {
  console.error(`Invalid ${app} Vercel environment:`);
  failures.forEach((failure) => console.error(`- ${failure}`));
  process.exit(1);
}

console.log(`${app} Vercel environment is valid for ${production ? "production" : "development"}.`);
