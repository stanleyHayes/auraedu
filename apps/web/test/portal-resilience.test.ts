import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const groups = ["(admin)", "(teacher)", "(parent)", "(student)", "(superadmin)", "(applicant)"];

void test("every authenticated portal has branded loading and recovery boundaries", () => {
  const app = join(import.meta.dirname, "..", "app");
  for (const group of groups) {
    for (const boundary of ["loading.tsx", "error.tsx"]) {
      assert.equal(existsSync(join(app, group, boundary)), true, `${group} is missing ${boundary}`);
    }
  }
  const loading = readFileSync(
    join(import.meta.dirname, "..", "components", "portal-route-loading.tsx"),
    "utf8",
  );
  const error = readFileSync(
    join(import.meta.dirname, "..", "components", "portal-route-error.tsx"),
    "utf8",
  );
  assert.match(loading, /aria-busy="true"/);
  assert.match(loading, /PageHeaderSkeleton/);
  assert.match(error, /role="alert"/);
  assert.match(error, /onClick=\{reset\}/);
});

void test("parent mobile brief links to each promised destination", () => {
  const page = readFileSync(
    join(import.meta.dirname, "..", "..", "mobile", "app", "(app)", "index.tsx"),
    "utf8",
  );
  for (const destination of ["/(app)/children", "/(app)/fees", "/(app)/report-cards"]) {
    assert.match(page, new RegExp(`href=["']${destination.replace(/[()]/g, "\\$&")}["']`));
  }
});

void test("enabled teacher mobile modules have real destinations", () => {
  const work = readFileSync(
    join(import.meta.dirname, "..", "..", "mobile", "app", "(app)", "work.tsx"),
    "utf8",
  );
  assert.match(work, /\["Classes", "academic_management", "\/\(app\)\/classes"\]/);
  assert.match(work, /\["Reports", "report_cards", "\/\(app\)\/report-cards"\]/);
  assert.equal(
    existsSync(join(import.meta.dirname, "..", "..", "mobile", "app", "(app)", "classes.tsx")),
    true,
  );
});
