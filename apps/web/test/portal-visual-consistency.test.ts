import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

function source(path: string) {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
}

const studentCollections = [
  "../app/(student)/student/assignments/page.tsx",
  "../app/(student)/student/cbt-exams/page.tsx",
  "../app/(student)/student/report-card/page.tsx",
  "../app/(student)/student/recommendations/page.tsx",
];

void test("student collection pages retain the unified portal hierarchy", () => {
  for (const path of studentCollections) {
    const page = source(path);
    assert.match(page, /<PageHeader\b/, `${path} needs the shared portal header`);
    assert.match(page, /<Reveal\b/, `${path} needs reduced-motion-safe content reveals`);
    assert.match(page, /<EmptyState\b/, `${path} needs a designed empty or failure state`);
  }
});

void test("student collection cards retain responsive editorial treatment", () => {
  for (const path of studentCollections) {
    const page = source(path);
    assert.match(page, /grid gap-4/, `${path} needs a responsive collection grid`);
    assert.match(page, /hover:-translate-y-0\.5/, `${path} needs restrained hover feedback`);
    assert.match(page, /border-\[var\(--border\)\]/, `${path} needs token-based borders`);
  }
});

function pageFiles(root: string): string[] {
  return readdirSync(root, { withFileTypes: true }).flatMap((entry) => {
    const path = join(root, entry.name);
    return entry.isDirectory() ? pageFiles(path) : entry.name === "page.tsx" ? [path] : [];
  });
}

void test("every protected web role enters through the unified portal shell", () => {
  const layouts = [
    "../app/(admin)/layout.tsx",
    "../app/(teacher)/layout.tsx",
    "../app/(parent)/layout.tsx",
    "../app/(student)/layout.tsx",
    "../app/(superadmin)/layout.tsx",
    "../app/(applicant)/layout.tsx",
  ];

  for (const path of layouts) {
    const layout = source(path);
    assert.match(
      layout,
      /(?:Admin|Teacher|Parent|Student|SuperAdmin)Shell/,
      `${path} must use a shared role shell`,
    );
    assert.match(layout, /showMobileMenu/, `${path} must expose the responsive navigation`);
  }
});

void test("every protected module page keeps a shared header or role dashboard", () => {
  const appRoot = fileURLToPath(new URL("../app", import.meta.url));
  const roots = ["(admin)", "(teacher)", "(parent)", "(student)", "(superadmin)", "(applicant)"];
  const pages = roots.flatMap((root) => pageFiles(join(appRoot, root)));

  for (const path of pages) {
    const page = readFileSync(path, "utf8");
    assert.match(
      page,
      /PageHeader|(?:Admin|Teacher|Parent|Student)Dashboard|portal-hero|<h1\b/,
      `${path} must retain the unified portal hierarchy`,
    );
  }
});

void test("tenant school websites retain the marketing-aligned public system", () => {
  const layout = source("../app/(public)/[tenant]/layout.tsx");
  const home = source("../app/(public)/[tenant]/page.tsx");
  const sections = source("../components/website-section.tsx");
  const assistant = source("../components/admissions-assistant.tsx");
  const styles = source("../app/globals.css");

  assert.match(layout, /school-site-header/);
  assert.match(layout, /AuraEduLogo/);
  assert.match(layout, /md:hidden/, "mobile visitors must retain a school-portal action");
  assert.match(home, /school-site-hero/, "an unpublished home page still needs a real hero");
  assert.match(home, /previewPaths/, "the preview must give visitors useful next steps");
  assert.match(home, /School-approved information/, "the preview must explain its trust model");
  assert.match(sections, /school-site-hero/);
  assert.match(sections, /school-site-card/);
  assert.match(sections, /<Reveal\b/);
  assert.match(assistant, /100dvh/, "the assistant must become a usable mobile sheet");
  assert.match(
    assistant,
    /transcriptRef/,
    "assistant state changes must keep the active step visible",
  );
  assert.match(
    assistant,
    /prefers-reduced-motion/,
    "assistant scrolling must respect reduced motion",
  );
  assert.match(styles, /prefers-reduced-motion/);
});

void test("staff administration includes the complete marketing-aligned people workflow", () => {
  const page = source("../app/(admin)/admin/staff/page.tsx");
  const workspace = source("../components/staff-assignment-workspace.tsx");
  const form = source("../components/staff-form.tsx");
  const sheet = source("../components/staff-form-sheet.tsx");
  const actions = source("../lib/staff-actions.ts");

  assert.match(
    page,
    /StaffFormSheet mode="create"/,
    "staff creation must be a real primary action",
  );
  assert.match(page, /StaffFormSheet mode="edit"/, "directory rows must support lifecycle edits");
  assert.match(workspace, /portal-accent-soft/, "assignment scope must retain portal theming");
  assert.match(
    workspace,
    /transition hover:-translate-y-0\.5/,
    "assignment cards need restrained motion",
  );
  assert.match(form, /name="status"/, "staff lifecycle must support active and inactive states");
  assert.match(
    form,
    /name="user_id"/,
    "staff records must link to an Identity account without raw entry",
  );
  assert.match(
    sheet,
    /motion|blur-2xl|portal-accent/,
    "the staff form needs the shared visual language",
  );
  assert.match(
    actions,
    /client\.post\("\/api\/v1\/staff"/,
    "staff creation must call the real API",
  );
  assert.match(actions, /client\.patch\(`\/api\/v1\/staff\//, "staff edits must call the real API");
});

void test("student administration is an operational enrolment workspace", () => {
  const page = source("../app/(admin)/admin/students/page.tsx");
  const form = source("../components/student-form.tsx");
  const sheet = source("../components/student-form-sheet.tsx");
  const actions = source("../lib/student-actions.ts");

  assert.match(page, /StudentFormSheet mode="create"/, "students need a real create action");
  assert.match(page, /StudentFormSheet\s+mode="edit"/, "student lifecycle must be editable");
  assert.match(page, /StatCard/, "students need operational summaries");
  assert.match(
    form,
    /name="academic_year_id"/,
    "creation must establish an academic-year enrolment",
  );
  assert.match(form, /name="class_id"/, "creation must establish class scope");
  assert.match(form, /name="user_id"/, "learner portal ownership must be linkable");
  assert.match(form, /name="status"/, "learner lifecycle status must be editable");
  assert.match(sheet, /portal-accent|blur-2xl/, "the enrolment sheet must share portal styling");
  assert.match(actions, /client\.post\("\/api\/v1\/students"/, "creation must use Student Service");
  assert.match(
    actions,
    /client\.patch\(`\/api\/v1\/students\//,
    "updates must use Student Service",
  );
});

void test("academic calendar manages years and their teaching terms", () => {
  const page = source("../app/(admin)/admin/academic-years/page.tsx");
  const sheet = source("../components/academic-calendar-form-sheet.tsx");
  const actions = source("../lib/academic-actions.ts");

  assert.match(page, /AcademicCalendarFormSheet kind="year" mode="create"/);
  assert.match(page, /AcademicCalendarFormSheet kind="term" mode="create"/);
  assert.match(page, /<Reveal\b/, "calendar surfaces need reduced-motion-safe reveals");
  assert.match(page, /StatCard/, "calendar needs operational context, not a flat table");
  assert.match(sheet, /name="is_current"/, "the current operating cycle must be configurable");
  assert.match(sheet, /name="academic_year_id"/, "every term must retain its owning year");
  assert.match(sheet, /min=\{selectedYear\?\.start_date\}/, "term dates must respect year bounds");
  assert.match(actions, /client\.post\("\/api\/v1\/academic-years"/);
  assert.match(actions, /client\.patch\(`\/api\/v1\/terms\//);
});

void test("school communications is a publishable role-aware workspace", () => {
  const page = source("../app/(admin)/admin/communications/page.tsx");
  const workflow = source("../components/announcement-workflow.tsx");
  const actions = source("../lib/communication-actions.ts");

  assert.match(page, /<AnnouncementFormSheet/, "communications need a publish action");
  assert.match(page, /StatCard/, "communications need operational context");
  assert.match(page, /<Reveal\b/, "communications need shared reduced-motion-safe reveals");
  assert.match(workflow, /name="audience"/, "publishing must choose an explicit audience");
  assert.match(workflow, /Parents and guardians/, "family-facing language must stay human");
  assert.match(actions, /client\.post\("\/api\/v1\/announcements"/);
  assert.match(actions, /client\.del\(`\/api\/v1\/announcements\//);
});
