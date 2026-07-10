import path from "node:path";
import type { NextConfig } from "next";

// Monorepo root — Turbopack compiles files inside this root, so the workspace
// design-system packages (packages/ui, packages/tokens) are reachable.
const repoRoot = path.resolve(import.meta.dirname, "../..");

const nextConfig: NextConfig = {
  transpilePackages: ["@auraedu/ui", "@auraedu/tokens"],
  output: "standalone",
  outputFileTracingRoot: repoRoot,
  turbopack: { root: repoRoot },
};

export default nextConfig;
