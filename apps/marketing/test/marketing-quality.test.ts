import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import test from "node:test";

function source(relative: string): string {
  return readFileSync(fileURLToPath(new URL(relative, import.meta.url)), "utf8");
}

void test("the marketing shell exposes one keyboard-navigable main landmark", () => {
  const layout = source("../app/layout.tsx");
  const contact = source("../app/contact/page.tsx");
  assert.match(layout, /href="#main-content"/);
  assert.match(layout, /<main id="main-content"/);
  assert.doesNotMatch(contact, /<main\b/);
});

void test("public trust statements are linked and discoverable", () => {
  const footer = source("../components/site-footer.tsx");
  const sitemap = source("../app/sitemap.ts");
  for (const route of ["/privacy", "/security", "/accessibility"]) {
    assert.match(footer, new RegExp(`\\["${route}"`));
    assert.match(sitemap, new RegExp(`"${route}"`));
  }
});

void test("client-only conversion pages retain server metadata", () => {
  for (const route of ["pricing", "contact"]) {
    const layout = source(`../app/${route}/layout.tsx`);
    assert.match(layout, /export const metadata/);
    assert.match(layout, /description:/);
  }
});

void test("the site ships a branded social preview and protects API routes from indexing", () => {
  assert.match(source("../app/opengraph-image.tsx"), /new ImageResponse/);
  assert.match(source("../app/robots.ts"), /disallow: "\/api\/"/);
});

void test("the marketing site applies baseline browser security headers to every route", () => {
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
