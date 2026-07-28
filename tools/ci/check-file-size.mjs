#!/usr/bin/env node

// File-size budget guard (adopt-gradually pattern): budgets apply to every
// tracked file; current violations live in a committed shrink-only allowlist
// (tools/ci/file-size-allowlist.txt) so the gate passes today, entries can
// only shrink, and any new or growing violation fails.
//
// Usage:
//   node tools/ci/check-file-size.mjs            enforce budgets
//   node tools/ci/check-file-size.mjs --write    regenerate the allowlist from current violations

import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const ALLOWLIST_PATH = "tools/ci/file-size-allowlist.txt";

const BUDGETS = [
  {
    label: "Go source",
    pattern: (path) => path.endsWith(".go") && !path.endsWith("_test.go"),
    max: 600,
  },
  { label: "Go test", pattern: (path) => path.endsWith("_test.go"), max: 800 },
  { label: "TS/TSX", pattern: (path) => /\.tsx?$/.test(path), max: 400 },
  { label: "mjs", pattern: (path) => path.endsWith(".mjs"), max: 400 },
];

const HEADER =
  "# Shrink-only file-size allowlist (node tools/ci/check-file-size.mjs --write).\n" +
  "# Format: <path>\\t<allowed lines>. Entries may only shrink; new or growing\n" +
  "# violations fail the gate. Delete an entry once the file fits its budget.\n";

function budgetFor(path) {
  return BUDGETS.find(({ pattern }) => pattern(path)) ?? null;
}

function trackedFiles() {
  return execFileSync("git", ["-C", ROOT, "ls-files"], { encoding: "utf8" })
    .split("\n")
    .filter((path) => path.length > 0 && path !== ALLOWLIST_PATH);
}

function lineCount(path) {
  const contents = readFileSync(resolve(ROOT, path), "utf8");
  if (contents.length === 0) return 0;
  return contents.endsWith("\n") ? contents.split("\n").length - 1 : contents.split("\n").length;
}

function currentViolations() {
  const violations = new Map();
  for (const path of trackedFiles()) {
    const budget = budgetFor(path);
    if (!budget) continue;
    const lines = lineCount(path);
    if (lines > budget.max) violations.set(path, { lines, max: budget.max, label: budget.label });
  }
  return violations;
}

function parseAllowlist() {
  const entries = new Map();
  let raw = "";
  try {
    raw = readFileSync(resolve(ROOT, ALLOWLIST_PATH), "utf8");
  } catch {
    return entries;
  }
  for (const line of raw.split("\n")) {
    if (line.trim() === "" || line.startsWith("#")) continue;
    const [path, count] = line.split("\t");
    if (!path || !/^\d+$/.test(count ?? "")) {
      throw new Error(`${ALLOWLIST_PATH}: malformed entry: ${line}`);
    }
    entries.set(path, Number(count));
  }
  return entries;
}

function writeAllowlist(violations) {
  const rows = [...violations.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([path, { lines }]) => `${path}\t${lines}`);
  writeFileSync(resolve(ROOT, ALLOWLIST_PATH), HEADER + rows.join("\n") + "\n");
  console.log(
    `Wrote ${rows.length} allowlist entr${rows.length === 1 ? "y" : "ies"} to ${ALLOWLIST_PATH}.`,
  );
}

export function check() {
  const violations = currentViolations();
  const allowlist = parseAllowlist();
  const errors = [];
  const stale = [];

  for (const [path, { lines, max, label }] of [...violations.entries()].sort(([a], [b]) =>
    a.localeCompare(b),
  )) {
    const allowed = allowlist.get(path);
    if (allowed === undefined) {
      errors.push(`${path}: ${lines} lines exceeds the ${label} budget of ${max} (new violation)`);
    } else if (lines > allowed) {
      errors.push(
        `${path}: grew from allowlisted ${allowed} to ${lines} lines (allowlist is shrink-only)`,
      );
    }
  }
  for (const [path] of allowlist) {
    if (!violations.has(path)) stale.push(path);
  }

  for (const path of stale) {
    console.warn(`warning: ${path} no longer violates its budget; prune its allowlist entry`);
  }
  if (errors.length > 0) {
    for (const error of errors) console.error(`file-size: ${error}`);
    return 1;
  }
  console.log(
    `File-size budgets hold: ${violations.size} allowlisted violation(s), none new, none growing.`,
  );
  return 0;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  if (process.argv.includes("--write")) {
    writeAllowlist(currentViolations());
  } else {
    process.exit(check());
  }
}
