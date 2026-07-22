#!/usr/bin/env node
/** Compare public Go HTTP registrations with their source OpenAPI operations. */
import { promises as fs } from "node:fs";
import type { Dirent } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import yaml from "js-yaml";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const appsDirectory = path.join(root, "apps");
const contractsDirectory = path.join(root, "contracts", "openapi");
const methods = new Set(["get", "post", "put", "patch", "delete"]);
const contractServiceAliases: Record<string, string> = {
  assistant: "ai-orchestrator",
};

interface OpenAPIDocument {
  servers?: Array<{ url?: string }>;
  paths?: Record<string, Record<string, unknown>>;
}

interface Drift {
  service: string;
  runtimeOnly: string[];
  contractOnly: string[];
}

function normalizeRoute(method: string, route: string): string {
  const normalizedPath = `/${route}`
    .replaceAll(/\/+/g, "/")
    .replace(/\/$/, "")
    .replaceAll(/\{[^}]+\}/g, "{}");
  return `${method.toUpperCase()} ${normalizedPath || "/"}`;
}

function joinRoute(base: string, route: string): string {
  if (!base || base === "/") return route;
  return `${base.replace(/\/$/, "")}/${route.replace(/^\//, "")}`;
}

async function goFiles(directory: string): Promise<string[]> {
  const found: string[] = [];
  let entries: Dirent[];
  try {
    entries = await fs.readdir(directory, { withFileTypes: true });
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return found;
    throw error;
  }
  for (const entry of entries) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) found.push(...(await goFiles(absolute)));
    else if (entry.isFile() && entry.name.endsWith(".go") && !entry.name.endsWith("_test.go"))
      found.push(absolute);
  }
  return found;
}

async function runtimeRoutes(service: string): Promise<Set<string>> {
  const directory = path.join(appsDirectory, `${service}-service`, "internal", "adapters", "http");
  const routes = new Set<string>();
  const registration = /\.HandleFunc\(\s*["`]\s*(GET|POST|PUT|PATCH|DELETE)\s+([^"`]+)["`]/g;
  for (const file of await goFiles(directory)) {
    const source = await fs.readFile(file, "utf8");
    for (const match of source.matchAll(registration)) {
      const method = match[1];
      const route = match[2];
      if (!method || !route || route.startsWith("/internal/")) continue;
      routes.add(normalizeRoute(method, route));
    }
  }
  return routes;
}

async function contractRoutes(file: string): Promise<Set<string>> {
  const document = yaml.load(await fs.readFile(file, "utf8")) as OpenAPIDocument;
  const server = document.servers?.[0]?.url ?? "";
  const routes = new Set<string>();
  for (const [route, operations] of Object.entries(document.paths ?? {})) {
    if (route.startsWith("/internal/")) continue;
    for (const method of Object.keys(operations)) {
      if (methods.has(method)) routes.add(normalizeRoute(method, joinRoute(server, route)));
    }
  }
  return routes;
}

async function main(): Promise<void> {
  const contractFiles = (await fs.readdir(contractsDirectory))
    .filter((file) => file.endsWith(".v1.yaml") && !file.includes("-internal."))
    .sort();
  const contractsByService = new Map<string, Set<string>>();
  for (const contractFile of contractFiles) {
    const contractName = contractFile.replace(/\.v1\.yaml$/, "");
    const service = contractServiceAliases[contractName] ?? contractName;
    const serviceDirectory = path.join(appsDirectory, `${service}-service`);
    try {
      await fs.access(serviceDirectory);
    } catch {
      continue;
    }
    const routes = contractsByService.get(service) ?? new Set<string>();
    for (const route of await contractRoutes(path.join(contractsDirectory, contractFile)))
      routes.add(route);
    contractsByService.set(service, routes);
  }

  const drift: Drift[] = [];
  let checked = 0;

  for (const [service, contract] of [...contractsByService].sort(([left], [right]) =>
    left.localeCompare(right),
  )) {
    const runtime = await runtimeRoutes(service);
    if (runtime.size === 0) continue;
    checked += 1;
    const runtimeOnly = [...runtime].filter((route) => !contract.has(route)).sort();
    const contractOnly = [...contract].filter((route) => !runtime.has(route)).sort();
    if (runtimeOnly.length > 0 || contractOnly.length > 0)
      drift.push({ service, runtimeOnly, contractOnly });
  }

  if (drift.length > 0) {
    const details = drift.flatMap(({ service, runtimeOnly, contractOnly }) => [
      `${service}:`,
      ...runtimeOnly.map((route) => `  runtime-only  ${route}`),
      ...contractOnly.map((route) => `  contract-only ${route}`),
    ]);
    throw new Error(
      `OpenAPI/runtime route drift across ${drift.length} services:\n${details.join("\n")}`,
    );
  }
  console.log(`Validated public OpenAPI route conformance for ${checked} Go services.`);
}

await main();
