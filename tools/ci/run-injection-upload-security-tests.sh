#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

go_bin=${GO_BIN:-}
if [[ -z "$go_bin" ]]; then
  go_bin=$(command -v go || true)
fi
if [[ -z "$go_bin" && -x /opt/homebrew/bin/go ]]; then
  go_bin=/opt/homebrew/bin/go
fi
if [[ -z "$go_bin" ]]; then
  echo "go executable not found" >&2
  exit 1
fi

export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/auraedu-injection-upload-go-cache}
mkdir -p "$GOCACHE"

echo "::group::file upload boundary tests"
(cd apps/file-service && GOWORK=off "$go_bin" test ./internal/application ./internal/adapters/storage ./tests/unit -count=1)
echo "::endgroup::"

echo "::group::real PostgreSQL injection probe"
(cd apps/crm-service && GOWORK=off "$go_bin" test ./tests/integration -run TestListLeadsSearchIsParameterBoundAgainstSQLInjection -count=1)
echo "::endgroup::"

echo "Injection and upload security matrix passed: SQL payload remains bound data; upload size, filename, tenant path, provider URL, signed folder/resource type and webhook public ID are constrained."
