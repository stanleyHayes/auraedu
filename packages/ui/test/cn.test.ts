import assert from "node:assert/strict";
import test from "node:test";

import { cn } from "../src/lib/cn.ts";

void test("cn composes conditional classes and resolves Tailwind conflicts", () => {
  assert.equal(cn("px-2", false, ["font-medium", { hidden: false }], "px-5"), "font-medium px-5");
  assert.equal(cn("text-sm text-red-500", "text-lg text-blue-600"), "text-lg text-blue-600");
});
