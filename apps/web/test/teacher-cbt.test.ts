import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(teacher)/teacher/cbt/page.tsx");
const detailPath = join(root, "app/(teacher)/teacher/cbt/[id]/page.tsx");
const actionsPath = join(root, "lib/teacher-cbt-actions.ts");
const page = readFileSync(pagePath, "utf8");
const detail = readFileSync(detailPath, "utf8");
const actions = readFileSync(actionsPath, "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

const components = [
  "teacher-cbt-exam-form.tsx",
  "teacher-cbt-exam-form-sheet.tsx",
  "teacher-cbt-exam-status-button.tsx",
  "teacher-cbt-delete-exam-button.tsx",
  "teacher-cbt-question-form.tsx",
  "teacher-cbt-question-form-sheet.tsx",
  "teacher-cbt-delete-question-button.tsx",
  "teacher-cbt-attach-question-button.tsx",
  "teacher-cbt-move-question-button.tsx",
];

void test("teacher CBT routes exist with loading states and navigation entry", () => {
  assert.ok(existsSync(pagePath), "CBT list page is missing");
  assert.ok(existsSync(detailPath), "CBT detail page is missing");
  assert.ok(existsSync(join(root, "app/(teacher)/teacher/cbt/loading.tsx")));
  assert.ok(existsSync(join(root, "app/(teacher)/teacher/cbt/[id]/loading.tsx")));
  assert.match(navigation, /href: "\/teacher\/cbt", feature: "cbt_exams"/);
  for (const component of components) {
    assert.ok(existsSync(join(root, "components", component)), `component ${component} missing`);
  }
  assert.ok(existsSync(join(root, "lib/teacher-cbt-utils.ts")), "shared helpers missing");
});

void test("CBT list page loads exams, submissions, and lookup data with honest states", () => {
  assert.match(page, /\/api\/v1\/cbt\/exams\?limit=50/);
  assert.match(page, /\/api\/v1\/cbt\/submissions\?limit=100/);
  assert.match(page, /\/teacher\/cbt\/\$\{exam\.id\}/);
  for (const column of ["Exam", "Subject / year", "Status", "Window", "Questions", "Submissions"]) {
    assert.match(page, new RegExp(`header: "${column.replace("/", "\\/")}"`), `column ${column}`);
  }
  assert.match(page, /Could not load exams/);
  assert.match(page, /No exams yet/);
  assert.match(page, /TeacherCbtExamFormSheet mode="create"/);
});

void test("exam actions cover the full authoring lifecycle against the contract", () => {
  assert.match(actions, /"use server"/);
  // Exam CRUD and lifecycle transitions via PATCH status (cbt.v1.yaml has no publish endpoint).
  assert.match(actions, /client\.post\("\/api\/v1\/cbt\/exams"/);
  assert.match(actions, /client\.patch\(`\/api\/v1\/cbt\/exams\/\$\{encodeURIComponent\(id\)\}`/);
  assert.match(actions, /client\.del\(`\/api\/v1\/cbt\/exams\/\$\{encodeURIComponent\(id\)\}`/);
  for (const status of ["draft", "published", "active", "closed", "archived"]) {
    assert.match(actions, new RegExp(`"${status}"`), `status ${status}`);
  }
  // Question CRUD.
  assert.match(actions, /client\.post(<[^>]+>)?\("\/api\/v1\/cbt\/questions"/);
  assert.match(
    actions,
    /client\.patch\(`\/api\/v1\/cbt\/questions\/\$\{encodeURIComponent\(questionId\)\}`/,
  );
  assert.match(
    actions,
    /client\.del\(`\/api\/v1\/cbt\/questions\/\$\{encodeURIComponent\(questionId\)\}`/,
  );
  // Attach, detach, and reorder all go through the exam's question_ids array.
  for (const name of [
    "createExamAction",
    "updateExamAction",
    "setExamStatusAction",
    "deleteExamAction",
    "createQuestionAction",
    "updateQuestionAction",
    "deleteQuestionAction",
    "attachQuestionAction",
    "detachQuestionAction",
    "moveQuestionAction",
  ]) {
    assert.match(actions, new RegExp(`export async function ${name}`), `action ${name}`);
  }
  assert.match(actions, /revalidatePath\("\/teacher\/cbt"\)/);
});

void test("question validation enforces the contract's types and answer rules", () => {
  const utils = readFileSync(join(root, "lib/teacher-cbt-utils.ts"), "utf8");
  for (const type of ["multiple_choice", "true_false", "short_answer"]) {
    assert.match(utils, new RegExp(`"${type}"`), `question type ${type}`);
  }
  assert.match(actions, /Multiple choice needs at least two options/);
  assert.match(actions, /The correct answer must match one of the options/);
  assert.match(actions, /Marks must be a whole number of at least 1/);
});

void test("CBT detail page authors questions, reorders them, and shows read-only submissions", () => {
  assert.match(detail, /\/api\/v1\/cbt\/exams\/\$\{encodeURIComponent\(id\)\}/);
  assert.match(detail, /\/api\/v1\/cbt\/questions\?limit=100/);
  assert.match(detail, /exam_id=\$\{encodeURIComponent\(id\)\}/);
  assert.match(detail, /Could not load the exam/);
  assert.match(detail, /Could not load questions/);
  assert.match(detail, /Could not load submissions/);
  assert.match(detail, /No questions yet/);
  assert.match(detail, /No submissions yet/);
  assert.match(detail, /Total marks/);
  assert.match(detail, /Available in the question bank/);
  for (const column of [
    "Question",
    "Type",
    "Marks",
    "Student",
    "Status",
    "Score",
    "Submitted at",
  ]) {
    assert.match(detail, new RegExp(`header: "${column}"`), `column ${column}`);
  }
  // Submissions stay read-only: only question/exam mutations exist, and those
  // live in the server actions module, not the page.
  assert.doesNotMatch(detail, /client\.(post|patch|put|del)\(/);
});

void test("question form supports the contract's question types", () => {
  const form = readFileSync(join(root, "components/teacher-cbt-question-form.tsx"), "utf8");
  assert.match(form, /name="question_text"/);
  assert.match(form, /name="question_type"/);
  assert.match(form, /name="correct_answer"/);
  assert.match(form, /name="marks"/);
  assert.match(form, /getAll\("option"\)|name="option"/);
  assert.match(form, /QUESTION_TYPES/);
  assert.match(form, /True/);
  assert.match(form, /False/);
});

void test("delete is offered for drafts only and status transitions are explicit", () => {
  assert.match(page, /exam\.status === "draft"/);
  const statusButton = readFileSync(
    join(root, "components/teacher-cbt-exam-status-button.tsx"),
    "utf8",
  );
  assert.match(statusButton, /target: "published"/);
  assert.match(statusButton, /target: "active"/);
  assert.match(statusButton, /target: "closed"/);
  assert.match(statusButton, /setExamStatusAction/);
});
