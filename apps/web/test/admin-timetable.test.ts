import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/timetable/page.tsx");
const actionsPath = join(root, "lib/admin-timetable-actions.ts");
const page = readFileSync(pagePath, "utf8");
const actions = readFileSync(actionsPath, "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

void test("timetable route exists with management components and navigation entry", () => {
  assert.ok(existsSync(pagePath), "timetable page is missing");
  assert.ok(existsSync(actionsPath), "timetable actions module is missing");
  for (const component of [
    "components/admin-timetable-entry-form.tsx",
    "components/admin-timetable-entry-sheet.tsx",
    "components/admin-timetable-delete-button.tsx",
  ]) {
    assert.ok(existsSync(join(root, component)), `${component} is missing`);
  }
  assert.match(navigation, /href: "\/admin\/timetable"/);
});

void test("timetable page filters by class and term and renders a weekly grid", () => {
  assert.match(page, /searchParams: Promise<\{ class_id\?: string; term_id\?: string \}>/);
  assert.match(page, /query\.set\("term_id", termId\)/);
  assert.match(page, /\/api\/v1\/timetable\?\$\{query\.toString\(\)\}/);
  assert.match(page, /\/api\/v1\/classes\?limit=50/);
  assert.match(page, /\/api\/v1\/subjects\?limit=100/);
  assert.match(page, /\/api\/v1\/staff\?limit=100/);
  assert.match(page, /Choose a class/);
  assert.match(page, /Could not load the timetable/);
  assert.match(page, /No periods scheduled/);
  for (const day of ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]) {
    assert.ok(page.includes(`"${day}"`), `weekday ${day} is missing`);
  }
});

void test("timetable actions follow the academic contract and surface conflicts", () => {
  assert.match(actions, /client\.post\("\/api\/v1\/timetable"/);
  assert.match(actions, /client\.patch\(`\/api\/v1\/timetable\/\$\{encodeURIComponent\(id\)\}`/);
  assert.match(actions, /client\.del\(`\/api\/v1\/timetable\/\$\{encodeURIComponent\(id\)\}`/);
  // 409 overlap responses are translated into a friendly conflict message
  assert.match(actions, /error\.status === 409/);
  assert.match(actions, /overlaps an existing period/);
  assert.match(actions, /revalidatePath\("\/admin\/timetable"\)/);
});

void test("entry form assigns subject, teacher, day, period and room per the schema", () => {
  const form = readFileSync(join(root, "components/admin-timetable-entry-form.tsx"), "utf8");
  for (const name of [
    'name="class_id"',
    'name="term_id"',
    'name="subject_id"',
    'name="teacher_id"',
    'name="weekday"',
    'name="start_time"',
    'name="end_time"',
    'name="room"',
  ]) {
    assert.ok(form.includes(name), `entry form is missing ${name}`);
  }
  // end time must follow start time before the API is called
  assert.match(actions, /endTime <= startTime/);
});
