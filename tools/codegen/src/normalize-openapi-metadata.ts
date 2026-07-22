import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { basename, resolve } from "node:path";

const methodPattern = /^(\s*)(get|put|post|delete|options|head|patch|trace):\s*$/;
const pathPattern = /^(\s+)(\/[^:]+):\s*$/;

function indentOf(line: string): number {
  return /^\s*/.exec(line)?.[0].length ?? 0;
}

function operationDescription(operationId: string): string {
  const words = operationId
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/([A-Z])([A-Z][a-z])/g, "$1 $2")
    .toLowerCase();
  return `Executes the ${words} workflow within this AuraEDU API boundary.`;
}

function operationTag(path: string, fallback: string): string {
  const ignored = new Set(["api", "v1", "internal", "public", "super-admin"]);
  return (
    path
      .split("/")
      .find((part) => part && !part.startsWith("{") && !ignored.has(part))
      ?.replace(/[^a-z0-9-]/gi, "-") ?? fallback
  );
}

function normalizeFile(path: string, write: boolean): number {
  const original = readFileSync(path, "utf8");
  const lines = original.replace(/\n$/, "").split("\n");
  const fallbackTag = basename(path).replace(/\.v\d+\.yaml$/, "");
  let changes = 0;

  // Remove metadata previously emitted by this normalizer so reruns can repair
  // indentation in older, unusually-indented contracts without duplication.
  for (let index = lines.length - 1; index >= 0; index -= 1) {
    if (lines[index]?.trimStart().startsWith("description: Executes the ")) {
      lines.splice(index, 1);
      changes += 1;
    }
    if (lines[index] === "  contact:" && lines[index + 1] === "    name: AuraEDU Engineering") {
      lines.splice(index, 2);
      changes += 2;
    }
    if (lines[index] === "servers: [{ url: / }]") {
      lines.splice(index, lines[index + 1] === "" ? 2 : 1);
      changes += 1;
    }
  }

  let currentPath = "";

  for (let index = 0; index < lines.length; index += 1) {
    const pathMatch = lines[index]?.match(pathPattern);
    if (pathMatch && indentOf(lines[index] ?? "") <= 4) currentPath = pathMatch[2] ?? "";

    const methodMatch = lines[index]?.match(methodPattern);
    if (!methodMatch) continue;
    const methodIndent = methodMatch[1]?.length ?? 0;
    let end = index + 1;
    while (end < lines.length) {
      const line = lines[end] ?? "";
      if (line.trim() && !line.trimStart().startsWith("#") && indentOf(line) <= methodIndent) break;
      end += 1;
    }
    let block = lines.slice(index + 1, end);
    const operationIdOffset = block.findIndex((line) =>
      line.trimStart().startsWith("operationId:"),
    );
    const propertyIndent =
      operationIdOffset >= 0 ? indentOf(block[operationIdOffset] ?? "") : methodIndent + 2;
    const childPrefix = " ".repeat(propertyIndent);
    const operationId =
      operationIdOffset >= 0 ? block[operationIdOffset]?.split(":", 2)[1]?.trim() : undefined;

    const tagOffsets = block
      .map((line, offset) => ({ line, offset }))
      .filter(({ line }) => line.trimStart().startsWith("tags:"));
    const preferredTag = tagOffsets.find(({ line }) => indentOf(line) === propertyIndent);
    for (const duplicate of tagOffsets
      .filter(({ offset }) => offset !== preferredTag?.offset)
      .sort((a, b) => b.offset - a.offset)) {
      lines.splice(index + 1 + duplicate.offset, 1);
      changes += 1;
    }

    end = index + 1;
    while (end < lines.length && (!lines[end]?.trim() || indentOf(lines[end] ?? "") > methodIndent))
      end += 1;
    block = lines.slice(index + 1, end);
    const refreshedOperationIdOffset = block.findIndex((line) =>
      line.trimStart().startsWith("operationId:"),
    );
    const hasDescription = block.some(
      (line) => indentOf(line) === propertyIndent && line.trimStart().startsWith("description:"),
    );
    const hasTags = block.some(
      (line) => indentOf(line) === propertyIndent && line.trimStart().startsWith("tags:"),
    );
    if (hasDescription && hasTags) continue;
    const insertionIndex =
      refreshedOperationIdOffset >= 0 ? index + 2 + refreshedOperationIdOffset : index + 1;
    const additions: string[] = [];
    if (!hasDescription)
      additions.push(
        `${childPrefix}description: ${operationDescription(operationId ?? `${methodMatch[2]} operation`)}`,
      );
    if (!hasTags)
      additions.push(`${childPrefix}tags: ['${operationTag(currentPath, fallbackTag)}']`);
    lines.splice(insertionIndex, 0, ...additions);
    changes += additions.length;
    index += additions.length;
  }

  const infoIndex = lines.findIndex((line) => line === "info:");
  if (infoIndex >= 0) {
    let infoEnd = infoIndex + 1;
    while (
      infoEnd < lines.length &&
      (!lines[infoEnd]?.trim() || indentOf(lines[infoEnd] ?? "") > 0)
    )
      infoEnd += 1;
    const infoBlock = lines.slice(infoIndex + 1, infoEnd);
    if (!infoBlock.some((line) => /^\s{2}contact:/.test(line))) {
      lines.splice(infoEnd, 0, "  contact:", "    name: AuraEDU Engineering");
      changes += 2;
    }
  }

  if (!lines.some((line) => line === "servers:" || /^servers:\s*\[/.test(line))) {
    const pathsIndex = lines.findIndex((line) => line === "paths:");
    if (pathsIndex >= 0) {
      lines.splice(pathsIndex, 0, "servers: [{ url: / }]", "");
      changes += 2;
    }
  }

  const operationTags = new Set(
    lines
      .map((line) => /^\s+tags:\s*\[['"]?([^'"\]]+)/.exec(line)?.[1]?.trim())
      .filter((tag): tag is string => Boolean(tag)),
  );
  const globalTagsIndex = lines.findIndex((line) => line === "tags:");
  const declaredTags = new Set<string>();
  let globalTagsEnd = globalTagsIndex;
  if (globalTagsIndex >= 0) {
    globalTagsEnd += 1;
    while (
      globalTagsEnd < lines.length &&
      (!lines[globalTagsEnd]?.trim() || indentOf(lines[globalTagsEnd] ?? "") > 0)
    ) {
      const match = lines[globalTagsEnd]?.match(/^\s+- name:\s*['"]?([^'"]+)/);
      if (match?.[1]) declaredTags.add(match[1].trim());
      globalTagsEnd += 1;
    }
  }
  const missingTags = [...operationTags].filter((tag) => !declaredTags.has(tag)).sort();
  if (missingTags.length > 0) {
    const declarations = missingTags.map((tag) => `  - name: ${tag}`);
    if (globalTagsIndex >= 0) {
      lines.splice(globalTagsEnd, 0, ...declarations);
    } else {
      const pathsIndex = lines.findIndex((line) => line === "paths:");
      lines.splice(pathsIndex, 0, "tags:", ...declarations, "");
    }
    changes += declarations.length + (globalTagsIndex >= 0 ? 0 : 2);
  }
  const normalized = `${lines.join("\n")}\n`;
  if (write && normalized !== original) writeFileSync(path, normalized);
  return normalized === original ? 0 : changes;
}

const write = process.argv.includes("--write");
const root = resolve(process.cwd(), "contracts/openapi");
const files = readdirSync(root)
  .filter((name) => name.endsWith(".yaml"))
  .map((name) => resolve(root, name));
const changes = files.reduce((count, file) => count + normalizeFile(file, write), 0);
if (!write && changes > 0) {
  throw new Error(
    `${changes} OpenAPI metadata lines are missing; run normalize-openapi-metadata.ts --write`,
  );
}
console.log(
  `${write ? "Normalized" : "Validated"} ${files.length} OpenAPI contracts (${changes} metadata lines ${write ? "added" : "missing"}).`,
);
