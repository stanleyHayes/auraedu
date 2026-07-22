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

export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/auraedu-authz-security-go-cache}
mkdir -p "$GOCACHE"

run_test() {
  local module=$1
  local package=$2
  local pattern=$3
  echo "::group::$module $package"
  (cd "$module" && GOWORK=off "$go_bin" test "$package" -run "$pattern" -count=1)
  echo "::endgroup::"
}

run_test platform ./auth 'Test'
run_test apps/api-gateway ./internal/gateway 'Test'
run_test apps/identity-service ./tests/unit 'Test'
run_test apps/tenant-service ./tests/unit 'Test'
run_test apps/academic-service ./tests/unit 'Test'
run_test apps/fees-service ./tests/unit 'Test'
run_test apps/student-service ./tests/unit 'Test'
run_test apps/payment-service ./tests/unit 'Test'

echo "AuthN/Z security matrix passed: token integrity and expiry, invalid-token rejection, tenant-claim binding, RBAC denial, internal-header stripping, service-token checks, and platform-admin boundaries."
