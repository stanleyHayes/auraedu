import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const page = readFileSync(join(root, "app/(admin)/admin/students/page.tsx"), "utf8");
const actions = readFileSync(join(root, "lib/admin-import-actions.ts"), "utf8");
const template = readFileSync(join(root, "lib/admin-import-template.ts"), "utf8");
const dialog = readFileSync(join(root, "components/admin-import-dialog.tsx"), "utf8");
const userGuide = readFileSync(join(root, "../../docs/user-guide.md"), "utf8");

void test("students page exposes the CSV import dialog next to manual creation", () => {
  assert.match(page, /AdminStudentImportDialog/);
  assert.match(page, /StudentFormSheet/);
});

void test("import action posts multipart to the student import endpoint with tenant context", () => {
  assert.match(actions, /"use server"/);
  assert.match(actions, /\/api\/v1\/students\/import/);
  assert.match(actions, /upload\.append\("file"/);
  assert.match(actions, /new File\(\[csvText\], "students\.csv"/);
  assert.match(actions, /tenantHeaderName/);
  assert.match(actions, /getCurrentToken/);
  assert.match(actions, /revalidatePath\("\/admin\/students"\)/);
});

void test("import action validates the CSV before uploading", () => {
  assert.match(actions, /first_name/);
  assert.match(actions, /last_name/);
  assert.match(actions, /MAX_CSV_BYTES/);
  assert.match(actions, /header row/);
});

void test("downloadable template uses the exact columns the student service parses", () => {
  for (const column of [
    "first_name",
    "last_name",
    "date_of_birth",
    "gender",
    "relationship",
    "guardian_first_name",
    "guardian_last_name",
    "guardian_phone",
    "guardian_email",
    "user_id",
    "guardian_user_id",
  ]) {
    assert.match(template, new RegExp(`"${column}"`), `column ${column} is missing`);
  }
  assert.match(template, /studentImportTemplateCsv/);
  assert.match(dialog, /students-import-template\.csv/);
});

void test("dialog previews rows client-side and renders per-row import errors", () => {
  assert.match(dialog, /previewCsv/);
  assert.match(dialog, /type="file"/);
  assert.match(dialog, /rows to import/);
  assert.match(dialog, /students_created/);
  assert.match(dialog, /guardians_created/);
  assert.match(dialog, /links_created/);
  assert.match(dialog, /Row \{error\.row\}/);
  assert.match(dialog, /role="alert"/);
});

void test("user guide promise about the CSV import dialog stays true", () => {
  assert.match(userGuide, /CSV bulk import/);
  assert.match(userGuide, /import dialog/);
});
