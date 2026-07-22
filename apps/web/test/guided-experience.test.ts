import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

import { PAGE_GUIDES, getPageGuide } from "../lib/page-guides.ts";

const root = join(import.meta.dirname, "..");

void test("the contextual guide registry covers every portal role", () => {
  assert.ok(PAGE_GUIDES.length >= 20);
  for (const role of ["admin", "teacher", "parent", "student", "superadmin", "applicant"]) {
    assert.ok(
      PAGE_GUIDES.some((guide) => guide.href === `/${role}`),
      `${role} guide missing`,
    );
  }
  for (const guide of PAGE_GUIDES) {
    assert.ok(guide.steps.length >= 3, `${guide.href} needs a complete walkthrough`);
    assert.equal(new Set(guide.steps).size, guide.steps.length, `${guide.href} repeats a step`);
  }
});

void test("page guide resolution prefers the most specific route and preserves live copy", () => {
  const guide = getPageGuide("/admin/students/learner-1", {
    title: "Learner record",
    description: "Verified school record.",
  });
  assert.equal(guide.href, "/admin/students");
  assert.equal(guide.title, "Learner record");
  assert.equal(guide.description, "Verified school record.");
});

void test("the portal mounts the tour, guide provider and complete anchor set", () => {
  const shell = readFileSync(join(root, "components", "portal-shell.tsx"), "utf8");
  const topbar = readFileSync(join(root, "components", "app-topbar.tsx"), "utf8");
  const tour = readFileSync(join(root, "components", "app-tour.tsx"), "utf8");
  const pageHeader = readFileSync(
    join(root, "..", "..", "packages", "ui", "src", "components", "page-header.tsx"),
    "utf8",
  );
  const userMenu = readFileSync(
    join(root, "..", "..", "packages", "ui", "src", "components", "user-menu.tsx"),
    "utf8",
  );

  assert.match(shell, /<PageGuideProvider/);
  assert.match(shell, /<AppTour/);
  for (const anchor of [
    "desktop-navigation",
    "theme-toggle",
    "notifications",
    "user-menu",
    "page-header",
    "primary-actions",
  ]) {
    assert.match(
      `${shell}\n${topbar}\n${pageHeader}\n${userMenu}`,
      new RegExp(`data-tour=["']${anchor}["']`),
    );
  }
  assert.match(tour, /auraedu-tour-complete:\$\{mode\}:\$\{tenantId\}:\$\{userId\}/);
  assert.match(tour, /prefers-reduced-motion: reduce/);
  assert.match(tour, /connection\?\.saveData/);
  assert.match(tour, /aria-modal="true"/);
});

void test("page help uses one guide for visible steps, transcript and speech", () => {
  const help = readFileSync(
    join(root, "..", "..", "packages", "ui", "src", "components", "page-guide.tsx"),
    "utf8",
  );
  assert.match(help, /data-page-guide/);
  assert.match(help, /guide\.steps\.map/g);
  assert.match(help, /SpeechSynthesisUtterance/);
  assert.match(help, /utterance\.lang = "en-GB"/);
  assert.match(help, /speechSynthesis\.cancel/);
});
