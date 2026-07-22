import assert from "node:assert/strict";
import { chmodSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { discoverServices, loadDatabaseUrls, validateInventory } from "./orchestrate.mjs";

test("every service migration directory has a supported contiguous runner", () => {
  const services = discoverServices();
  assert.equal(services.length, 26);
  assert.deepEqual(validateInventory(services), []);
  assert.equal(
    services.reduce((sum, item) => sum + item.migrations.length, 0),
    127,
  );
  assert.deepEqual(
    services.filter((item) => item.runner === "python").map((item) => item.service),
    ["ai-prediction-service", "ai-recommendation-service", "career-guidance-service"],
  );
});

test("database URL maps are private, complete, and PostgreSQL-only", () => {
  const directory = mkdtempSync(join(tmpdir(), "auraedu-migration-test-"));
  const path = join(directory, "urls.json");
  const selected = [{ service: "student-service" }];
  writeFileSync(path, JSON.stringify({ "student-service": "postgresql://db.example/student" }));
  chmodSync(path, 0o600);
  assert.equal(
    loadDatabaseUrls(path, selected)["student-service"],
    "postgresql://db.example/student",
  );

  chmodSync(path, 0o644);
  assert.throws(() => loadDatabaseUrls(path, selected), /chmod 600/);
  chmodSync(path, 0o600);
  writeFileSync(path, JSON.stringify({ "student-service": "https://db.example/student" }));
  assert.throws(() => loadDatabaseUrls(path, selected), /must use postgres/);
  writeFileSync(path, "{}");
  assert.throws(() => loadDatabaseUrls(path, selected), /database URL is missing/);
});
