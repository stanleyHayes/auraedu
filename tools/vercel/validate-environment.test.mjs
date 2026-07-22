/* global process, URL */
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import test from "node:test";

const script = new URL("./validate-environment.mjs", import.meta.url);

function run(app, env = {}) {
  return spawnSync(process.execPath, [script.pathname, app], {
    env: { PATH: process.env.PATH, ...env },
    encoding: "utf8",
  });
}

test("production requires clean HTTPS API and app origins", () => {
  const missing = run("web", { VERCEL_ENV: "production" });
  assert.equal(missing.status, 1);
  assert.match(missing.stderr, /NEXT_PUBLIC_API_URL is required/);
  assert.match(missing.stderr, /NEXT_PUBLIC_APP_URL is required/);

  const unsafe = run("web", {
    VERCEL_ENV: "production",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NEXT_PUBLIC_APP_URL: "https://user:pass@example.com/path?token=x",
  });
  assert.equal(unsafe.status, 1);
  assert.match(unsafe.stderr, /non-loopback HTTPS origin/);
  assert.match(unsafe.stderr, /without credentials/);
  assert.match(unsafe.stderr, /must not contain a path/);
});

test("production accepts clean web and marketing origins", () => {
  const env = {
    VERCEL_ENV: "production",
    NEXT_PUBLIC_API_URL: "https://api.auraedu.example",
    NEXT_PUBLIC_APP_URL: "https://auraedugh.vercel.app",
  };
  assert.equal(run("web", env).status, 0);
  assert.equal(
    run("marketing", { ...env, AURAEDU_API_URL: "https://api.auraedu.example" }).status,
    0,
  );
});

test("unknown frontend fails closed", () => {
  const result = run("unknown");
  assert.equal(result.status, 2);
  assert.match(result.stderr, /Usage/);
});
