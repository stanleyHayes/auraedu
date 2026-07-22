import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

const mobileRoot = fileURLToPath(new URL("..", import.meta.url));
interface NativeReleaseConfig {
  ios: {
    privacyManifests: {
      NSPrivacyTracking: boolean;
      NSPrivacyTrackingDomains: string[];
      NSPrivacyAccessedAPITypes: { NSPrivacyAccessedAPIType: string }[];
    };
  };
  android: { blockedPermissions: string[] };
}
const app = (
  JSON.parse(readFileSync(`${mobileRoot}/app.json`, "utf8")) as {
    expo: NativeReleaseConfig;
  }
).expo;

void test("native release config declares required-reason APIs and blocks unused sensors", () => {
  const categories = new Set(
    app.ios.privacyManifests.NSPrivacyAccessedAPITypes.map(
      (entry: { NSPrivacyAccessedAPIType: string }) => entry.NSPrivacyAccessedAPIType,
    ),
  );
  assert.deepEqual(
    categories,
    new Set([
      "NSPrivacyAccessedAPICategoryUserDefaults",
      "NSPrivacyAccessedAPICategoryFileTimestamp",
      "NSPrivacyAccessedAPICategoryDiskSpace",
      "NSPrivacyAccessedAPICategorySystemBootTime",
    ]),
  );
  assert.equal(app.ios.privacyManifests.NSPrivacyTracking, false);
  assert.deepEqual(app.ios.privacyManifests.NSPrivacyTrackingDomains, []);

  const blocked = new Set(app.android.blockedPermissions);
  for (const permission of [
    "android.permission.ACCESS_FINE_LOCATION",
    "android.permission.CAMERA",
    "android.permission.READ_CONTACTS",
    "android.permission.RECORD_AUDIO",
    "com.google.android.gms.permission.AD_ID",
  ]) {
    assert.equal(blocked.has(permission), true, permission);
  }
});

void test("push permission is user initiated instead of requested after sign in", () => {
  const source = readFileSync(`${mobileRoot}/src/auth.tsx`, "utf8");
  assert.match(source, /registerPush\(session, authenticatedFetch, false\)/);
  assert.match(source, /registerPush\(active, authenticatedFetch, true\)/);
  assert.match(source, /requestPermission &&\s*permission\.canAskAgain/);
});

void test("downloaded report cards are removed from cache after the share flow", () => {
  const source = readFileSync(`${mobileRoot}/app/(app)/report-cards.tsx`, "utf8");
  assert.match(
    source,
    /finally \{\s*await FileSystem\.deleteAsync\(localUri, \{ idempotent: true \}\)/,
  );
});
