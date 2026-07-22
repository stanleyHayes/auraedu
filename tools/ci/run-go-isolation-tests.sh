#!/usr/bin/env bash
set -euo pipefail

test_pattern='Isolation|RLS|CrossTenant'
go_bin="${GO_BIN:-$(command -v go || true)}"
if [[ -z "$go_bin" && -x /opt/homebrew/bin/go ]]; then
  go_bin=/opt/homebrew/bin/go
fi
if [[ -z "$go_bin" ]]; then
  echo "ERROR: Go is required for the tenant-isolation matrix." >&2
  exit 1
fi
matrix_tmp="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-isolation.XXXXXX")"
trap 'rm -rf "$matrix_tmp"' EXIT
inventory="$matrix_tmp/tenant-tables"

AURA_RLS_INVENTORY_FILE="$inventory" bash tools/ci/check-tenant-rls.sh

services=()
while IFS= read -r directory; do
  services+=("$directory")
done < <(
  cut -d'|' -f1 "$inventory" |
    sed 's|/migrations$||' |
    sort -u |
    while IFS= read -r directory; do
      [[ -f "$directory/go.mod" ]] && printf '%s\n' "$directory"
    done
)

failed=0
for directory in "${services[@]}"; do
  if ! rg -q "^func Test.*($test_pattern)" "$directory/tests/integration" --glob '*_test.go' 2>/dev/null; then
    echo "ERROR: $directory owns tenant tables but has no named integration isolation/RLS test." >&2
    failed=1
  fi
done
if [[ "$failed" != "0" ]]; then
  exit 1
fi

echo "Running dedicated Go tenant-isolation matrix for ${#services[@]} services."
for directory in "${services[@]}"; do
  echo "::group::$directory"
  (
    cd "$directory"
    GOWORK=off "$go_bin" test ./tests/integration -run "$test_pattern" -count=1
  )
  echo "::endgroup::"
done

echo "Go tenant-isolation matrix passed for ${#services[@]} tenant-owning services."
