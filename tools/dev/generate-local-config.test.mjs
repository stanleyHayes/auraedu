import assert from "node:assert/strict";
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import test from "node:test";

const sourceScript = join(dirname(fileURLToPath(import.meta.url)), "generate-local-config.mjs");

function runGenerator(t, initialEnv) {
  const root = mkdtempSync(join(tmpdir(), "auraedu-local-config-"));
  t.after(() => rmSync(root, { force: true, recursive: true }));

  const script = join(root, "tools", "dev", "generate-local-config.mjs");
  mkdirSync(dirname(script), { recursive: true });
  mkdirSync(join(root, "apps", "mobile"), { recursive: true });
  copyFileSync(sourceScript, script);
  writeFileSync(join(root, ".env"), initialEnv, { mode: 0o600 });

  const result = spawnSync(process.execPath, [script], {
    cwd: root,
    encoding: "utf8",
  });
  assert.equal(result.status, 0, result.stderr);
  return {
    content: readFileSync(join(root, ".env"), "utf8"),
    output: `${result.stdout}\n${result.stderr}`,
  };
}

test("Resend mode safely promotes an API key stored in the SMTP password slot", (t) => {
  const secret = "re_test_private_value";
  const result = runGenerator(
    t,
    `NOTIFICATION_PROVIDER=resend\nSMTP_PASSWORD=${secret}\nRESEND_API_KEY=\n`,
  );

  assert.match(result.content, new RegExp(`^RESEND_API_KEY=${secret}$`, "m"));
  assert.doesNotMatch(result.output, new RegExp(secret));
  assert.match(result.output, /moved from the SMTP password slot without printing it/);
});

test("existing Resend credentials and non-Resend providers are preserved", (t) => {
  const existing = runGenerator(
    t,
    "NOTIFICATION_PROVIDER=resend\nSMTP_PASSWORD=smtp-value\nRESEND_API_KEY=existing-value\n",
  );
  assert.match(existing.content, /^RESEND_API_KEY=existing-value$/m);

  const smtp = runGenerator(
    t,
    "NOTIFICATION_PROVIDER=smtp\nSMTP_PASSWORD=smtp-value\nRESEND_API_KEY=\n",
  );
  assert.match(smtp.content, /^RESEND_API_KEY=$/m);
});
