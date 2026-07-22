#!/usr/bin/env node

import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const PYTHON_MODULES = new Map([
  ["ai-prediction-service", "ai_prediction_service.migrate"],
  ["ai-recommendation-service", "ai_recommendation_service.migrate"],
  ["career-guidance-service", "career_guidance_service.migrate"],
]);

export function discoverServices(root = ROOT) {
  return readdirSync(join(root, "apps"), { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && entry.name.endsWith("-service"))
    .map((entry) => {
      const service = entry.name;
      const appDir = join(root, "apps", service);
      const migrationsDir = join(appDir, "migrations");
      if (!existsSync(migrationsDir)) return null;
      const migrations = readdirSync(migrationsDir)
        .filter((name) => name.endsWith(".sql"))
        .sort();
      if (migrations.length === 0) return null;
      const goMain = join(appDir, "cmd", service, "main.go");
      const pythonModule = PYTHON_MODULES.get(service);
      return {
        service,
        appDir,
        migrationsDir,
        migrations,
        runner: existsSync(goMain) ? "go" : pythonModule ? "python" : null,
        pythonModule,
      };
    })
    .filter(Boolean)
    .sort((a, b) => a.service.localeCompare(b.service));
}

export function validateInventory(services) {
  const errors = [];
  for (const item of services) {
    if (!item.runner) errors.push(`${item.service}: no supported migration runner`);
    const versions = item.migrations
      .map((name) => {
        const match = /^(\d{4})_[a-z0-9_]+\.sql$/.exec(name);
        if (!match) {
          errors.push(`${item.service}: invalid migration filename ${name}`);
          return null;
        }
        const sql = readFileSync(join(item.migrationsDir, name), "utf8");
        const legacyIdentityBaseline =
          item.service === "identity-service" && name === "0001_init.sql";
        if (!legacyIdentityBaseline && !/^-- \+goose Up\s*$/m.test(sql))
          errors.push(`${item.service}/${name}: missing '-- +goose Up' marker`);
        if (sql.trim().length < 20) errors.push(`${item.service}/${name}: migration is empty`);
        return Number(match[1]);
      })
      .filter((value) => value !== null);
    const unique = new Set(versions);
    if (unique.size !== versions.length)
      errors.push(`${item.service}: duplicate migration version`);
    versions.forEach((version, index) => {
      if (version !== index + 1)
        errors.push(
          `${item.service}: expected version ${String(index + 1).padStart(4, "0")}, found ${String(version).padStart(4, "0")}`,
        );
    });
  }
  return errors;
}

function parseArgs(argv) {
  const args = { check: false, dryRun: false, services: [] };
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (value === "--check") args.check = true;
    else if (value === "--dry-run") args.dryRun = true;
    else if (value === "--service") args.services.push(argv[++index] ?? "");
    else throw new Error(`unknown argument: ${value}`);
  }
  if (args.services.some((service) => !service)) throw new Error("--service requires a value");
  return args;
}

export function loadDatabaseUrls(path, selected) {
  if (!path)
    throw new Error(
      "AURA_MIGRATION_DATABASE_URLS_FILE is required unless --check or --dry-run is used",
    );
  const absolute = resolve(path);
  const mode = statSync(absolute).mode & 0o777;
  if ((mode & 0o077) !== 0)
    throw new Error(
      "database URL file must not be readable or writable by group/others (use chmod 600)",
    );
  const repoRelative = relative(ROOT, absolute);
  if (!repoRelative.startsWith("..") && !isAbsolute(repoRelative)) {
    try {
      execFileSync("git", ["-C", ROOT, "ls-files", "--error-unmatch", repoRelative], {
        stdio: "ignore",
      });
      throw new Error("database URL file must never be tracked by git");
    } catch (error) {
      if (error instanceof Error && error.message.includes("must never")) throw error;
    }
  }
  const parsed = JSON.parse(readFileSync(absolute, "utf8"));
  if (!parsed || Array.isArray(parsed) || typeof parsed !== "object")
    throw new Error("database URL file must contain a service-to-URL object");
  for (const item of selected) {
    const raw = parsed[item.service];
    if (typeof raw !== "string" || raw.trim() === "")
      throw new Error(`${item.service}: database URL is missing`);
    const url = new URL(raw);
    if (url.protocol !== "postgres:" && url.protocol !== "postgresql:")
      throw new Error(`${item.service}: database URL must use postgres:// or postgresql://`);
    if (!url.hostname || !url.pathname || url.pathname === "/")
      throw new Error(`${item.service}: database URL must name a host and database`);
  }
  return parsed;
}

function commandFor(item) {
  if (item.runner === "go")
    return { command: "go", args: ["run", `./cmd/${item.service}`, "migrate"] };
  return { command: "uv", args: ["run", "python", "-m", item.pythonModule] };
}

export function main(argv = process.argv.slice(2)) {
  const args = parseArgs(argv);
  const inventory = discoverServices();
  const errors = validateInventory(inventory);
  if (errors.length > 0)
    throw new Error(`migration inventory is invalid:\n- ${errors.join("\n- ")}`);
  const known = new Set(inventory.map((item) => item.service));
  for (const requested of args.services)
    if (!known.has(requested)) throw new Error(`unknown or migration-free service: ${requested}`);
  const selected =
    args.services.length > 0
      ? inventory.filter((item) => args.services.includes(item.service))
      : inventory;

  console.log(
    `Migration inventory valid: ${inventory.length} services, ${inventory.reduce((sum, item) => sum + item.migrations.length, 0)} versioned SQL files.`,
  );
  for (const item of selected) {
    const last = item.migrations.at(-1);
    console.log(
      `[plan] ${item.service}: ${item.migrations.length} migrations through ${last} via ${item.runner}`,
    );
  }
  if (args.check || args.dryRun) return;

  const urls = loadDatabaseUrls(process.env.AURA_MIGRATION_DATABASE_URLS_FILE, selected);
  for (const item of selected) {
    const { command, args: commandArgs } = commandFor(item);
    console.log(`[apply] ${item.service}`);
    const result = spawnSync(command, commandArgs, {
      cwd: item.appDir,
      env: {
        ...process.env,
        DATABASE_URL: urls[item.service],
        MIGRATIONS_PATH: item.migrationsDir,
        GOWORK: "off",
      },
      stdio: "inherit",
      timeout: 5 * 60 * 1000,
    });
    if (result.error)
      throw new Error(`${item.service}: migration runner failed: ${result.error.message}`);
    if (result.status !== 0)
      throw new Error(`${item.service}: migration runner exited ${result.status}`);
  }
  console.log(`Applied migrations for ${selected.length} services.`);
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  try {
    main();
  } catch (error) {
    console.error(`ERROR: ${error instanceof Error ? error.message : String(error)}`);
    process.exitCode = 1;
  }
}
