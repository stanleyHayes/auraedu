// @ts-check
import { fileURLToPath } from "node:url";
import js from "@eslint/js";
import ts from "typescript-eslint";

const repoRoot = fileURLToPath(new URL("../..", import.meta.url));

/** @type {import("eslint").Linter.Config[]} */
export default ts.config(
  {
    name: "@auraedu/eslint-config/ignores",
    ignores: [
      "**/node_modules/**",
      "**/.next/**",
      "**/dist/**",
      "packages/shared-types/gen/**",
      "packages/shared-types/src/generated/**",
      "apps/*/dist/**",
    ],
  },
  js.configs.recommended,
  ts.configs.recommendedTypeChecked,
  ts.configs.stylisticTypeChecked,
  {
    name: "@auraedu/eslint-config/parser",
    languageOptions: {
      parserOptions: {
        project: true,
        tsconfigRootDir: repoRoot,
      },
    },
  },
  {
    name: "@auraedu/eslint-config/codegen",
    files: ["tools/codegen/src/**/*"],
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-return": "off",
      "@typescript-eslint/prefer-optional-chain": "off",
      "@typescript-eslint/array-type": "off",
    },
  },
  {
    name: "@auraedu/eslint-config/js",
    files: ["**/*.js", "**/*.cjs", "**/*.mjs"],
    ...ts.configs.disableTypeChecked,
  },
);
