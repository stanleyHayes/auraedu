import path from "node:path";
import type { NextConfig } from "next";

const repoRoot = path.resolve(import.meta.dirname, "../..");

const nextConfig: NextConfig = {
  transpilePackages: [
    "@auraedu/ui",
    "@auraedu/tokens",
    "@auraedu/config",
    "@auraedu/logger",
    "@auraedu/flags",
    "@auraedu/api-client",
    "@auraedu/shared-types",
  ],
  output: "standalone",
  outputFileTracingRoot: repoRoot,
  turbopack: { root: repoRoot },
};

export default nextConfig;
