#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

if rg -n --glob 'README.md' 'Scaffold placeholder' .; then
  echo "Documentation freshness failed: scaffold README text remains." >&2
  exit 1
fi

required_readmes=(
  apps/analytics-service/README.md
  packages/api-client/README.md
  packages/config/README.md
  packages/eslint-config/README.md
  packages/feature-flags/README.md
  packages/logger/README.md
  packages/shared-types/README.md
  packages/tokens/README.md
  packages/ui/README.md
  packages/ui-native/README.md
  platform/auth/README.md
  platform/db/README.md
  platform/eventbus/README.md
  platform/flags/README.md
  platform/tenancy/README.md
)

for readme in "${required_readmes[@]}"; do
  if [[ ! -s "$readme" ]] || [[ "$(wc -w < "$readme")" -lt 20 ]]; then
    echo "Documentation freshness failed: $readme is absent or not substantive." >&2
    exit 1
  fi
done

test -f packages/ui-native/package.json
test -f packages/ui-native/src/index.ts
rg -q '"name": "@auraedu/ui-native"' packages/ui-native/package.json
rg -q '"@auraedu/ui-native": "workspace:\*"' apps/mobile/package.json
rg -q 'from "@auraedu/ui-native"' apps/mobile/src/components.tsx
rg -q 'from "@auraedu/ui-native"' apps/mobile/src/theme.ts

echo "Documentation freshness: ${#required_readmes[@]} ownership guides and the shared native package boundary are current."
