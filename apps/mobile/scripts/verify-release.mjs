import console from "node:console";
import { access, readFile } from "node:fs/promises";
import { resolve } from "node:path";
import process from "node:process";
import { URL } from "node:url";

const root = resolve(import.meta.dirname, "../../..");
const app = JSON.parse(await readFile(resolve(root, "apps/mobile/app.json"), "utf8")).expo;
const mobilePackage = JSON.parse(await readFile(resolve(root, "apps/mobile/package.json"), "utf8"));
const eas = JSON.parse(await readFile(resolve(root, "eas.json"), "utf8"));
const failures = [];

if (process.env.APP_ENV !== "production") failures.push("APP_ENV must be production.");

const apiUrl = process.env.EXPO_PUBLIC_API_URL?.trim() || "";
try {
  const parsed = new URL(apiUrl);
  if (parsed.protocol !== "https:") failures.push("EXPO_PUBLIC_API_URL must use HTTPS.");
  if (parsed.username || parsed.password || parsed.search || parsed.hash) {
    failures.push(
      "EXPO_PUBLIC_API_URL must not contain credentials, query parameters, or a fragment.",
    );
  }
} catch {
  failures.push("EXPO_PUBLIC_API_URL must be an absolute production URL.");
}

const projectId = process.env.EAS_PROJECT_ID?.trim() || "";
if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(projectId)) {
  failures.push("EAS_PROJECT_ID must be the linked Expo project UUID.");
}

if (!/^\d+\.\d+\.\d+$/.test(app.version) || app.version.startsWith("0.")) {
  failures.push("The public app version must be a stable semantic version.");
}
if (app.ios?.bundleIdentifier !== "com.auraedu.mobile") {
  failures.push("The iOS bundle identifier must remain com.auraedu.mobile.");
}
if (app.android?.package !== "com.auraedu.mobile") {
  failures.push("The Android package must remain com.auraedu.mobile.");
}
if (app.ios?.config?.usesNonExemptEncryption !== false) {
  failures.push("The reviewed non-exempt encryption declaration is missing.");
}
if (!eas.cli?.requireCommit) failures.push("EAS release builds must require a committed worktree.");
if (JSON.stringify(eas).includes("TODO"))
  failures.push("eas.json still contains placeholder values.");
if (mobilePackage.dependencies?.["expo-updates"] !== "~57.0.8") {
  failures.push("expo-updates must use the Expo SDK-compatible pinned range.");
}
if (app.runtimeVersion?.policy !== "appVersion") {
  failures.push("OTA updates must be isolated by the public native app version.");
}

const expectedPrivacyReasons = new Map([
  ["NSPrivacyAccessedAPICategoryUserDefaults", ["CA92.1"]],
  ["NSPrivacyAccessedAPICategoryFileTimestamp", ["0A2A.1", "3B52.1", "C617.1"]],
  ["NSPrivacyAccessedAPICategoryDiskSpace", ["85F4.1", "E174.1"]],
  ["NSPrivacyAccessedAPICategorySystemBootTime", ["35F9.1"]],
]);
const privacyManifest = app.ios?.privacyManifests;
if (privacyManifest?.NSPrivacyTracking !== false) {
  failures.push(
    "The iOS privacy manifest must explicitly declare that AuraEDU does not track users.",
  );
}
if (
  !Array.isArray(privacyManifest?.NSPrivacyTrackingDomains) ||
  privacyManifest.NSPrivacyTrackingDomains.length > 0
) {
  failures.push("The iOS privacy manifest must declare no tracking domains.");
}
const configuredPrivacyReasons = new Map(
  (privacyManifest?.NSPrivacyAccessedAPITypes ?? []).map((entry) => [
    entry.NSPrivacyAccessedAPIType,
    [...(entry.NSPrivacyAccessedAPITypeReasons ?? [])].sort(),
  ]),
);
for (const [category, reasons] of expectedPrivacyReasons) {
  if (
    JSON.stringify(configuredPrivacyReasons.get(category)) !== JSON.stringify([...reasons].sort())
  ) {
    failures.push(`The iOS privacy manifest is missing the reviewed ${category} reasons.`);
  }
}

const blockedAndroidPermissions = new Set(app.android?.blockedPermissions ?? []);
for (const permission of [
  "android.permission.ACCESS_COARSE_LOCATION",
  "android.permission.ACCESS_FINE_LOCATION",
  "android.permission.CAMERA",
  "android.permission.READ_CONTACTS",
  "android.permission.READ_MEDIA_IMAGES",
  "android.permission.READ_MEDIA_VIDEO",
  "android.permission.RECORD_AUDIO",
  "android.permission.WRITE_CONTACTS",
  "com.google.android.gms.permission.AD_ID",
]) {
  if (!blockedAndroidPermissions.has(permission)) {
    failures.push(`Unused sensitive Android permission must remain blocked: ${permission}.`);
  }
}

const expectedProfiles = {
  development: { channel: "development", environment: "development", appEnv: "development" },
  preview: { channel: "preview", environment: "preview", appEnv: "staging" },
  production: { channel: "production", environment: "production", appEnv: "production" },
};
for (const [name, expected] of Object.entries(expectedProfiles)) {
  const profile = eas.build?.[name];
  if (
    profile?.channel !== expected.channel ||
    profile?.environment !== expected.environment ||
    profile?.env?.APP_ENV !== expected.appEnv
  ) {
    failures.push(`${name} must use its isolated EAS channel and environment.`);
  }
}

const requiredAssets = new Map([
  ["apps/mobile/assets/icon.png", [1024, 1024]],
  ["apps/mobile/assets/adaptive-icon.png", [1024, 1024]],
  ["apps/mobile/assets/auraedu-logo.png", [416, 96]],
]);
for (const [relative, [expectedWidth, expectedHeight]] of requiredAssets) {
  try {
    const path = resolve(root, relative);
    await access(path);
    const png = await readFile(path);
    const signature = png.subarray(0, 8).toString("hex");
    if (signature !== "89504e470d0a1a0a") {
      failures.push(`${relative} must be a valid PNG asset.`);
      continue;
    }
    const width = png.readUInt32BE(16);
    const height = png.readUInt32BE(20);
    if (width !== expectedWidth || height !== expectedHeight) {
      failures.push(
        `${relative} must be ${expectedWidth}x${expectedHeight}; received ${width}x${height}.`,
      );
    }
  } catch {
    failures.push(`${relative} is missing.`);
  }
}

if (failures.length > 0) {
  console.error("AuraEDU mobile release configuration is incomplete:\n");
  failures.forEach((failure) => console.error(`- ${failure}`));
  process.exitCode = 1;
} else {
  console.log(
    "AuraEDU mobile release configuration is ready for an authenticated EAS production build.",
  );
}
