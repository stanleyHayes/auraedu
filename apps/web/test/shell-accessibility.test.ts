import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import test from "node:test";

function source(relative: string): string {
  return readFileSync(fileURLToPath(new URL(relative, import.meta.url)), "utf8");
}

void test("every navigation-heavy web shell can skip directly to its main landmark", () => {
  const portal = source("../components/portal-shell.tsx");
  const schoolSite = source("../app/(public)/[tenant]/layout.tsx");

  assert.match(portal, /href="#portal-main"/);
  assert.match(portal, /id="portal-main"/);
  assert.match(portal, /tabIndex=\{-1\}/);

  assert.match(schoolSite, /href="#school-site-main"/);
  assert.match(schoolSite, /id="school-site-main"/);
  assert.match(schoolSite, /tabIndex=\{-1\}/);
});

void test("skip links become visible on keyboard focus", () => {
  const styles = source("../app/globals.css");
  assert.match(styles, /\.app-skip-link:focus/);
  assert.match(styles, /transform: translateY\(0\)/);
});

void test("the portal applies baseline browser security headers to every route", () => {
  const config = source("../next.config.ts");
  for (const header of [
    "Strict-Transport-Security",
    "X-Content-Type-Options",
    "X-Frame-Options",
    "Referrer-Policy",
    "Cross-Origin-Opener-Policy",
    "Permissions-Policy",
  ]) {
    assert.match(config, new RegExp(`key: "${header}"`));
  }
  assert.match(config, /source: "\/\(\.\*\)"/);
});
