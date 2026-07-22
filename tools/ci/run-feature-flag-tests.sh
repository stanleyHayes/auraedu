#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/auraedu-feature-flags-go-cache}
mkdir -p "$GOCACHE"

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

node_bin=${NODE_BIN:-}
if [[ -z "$node_bin" ]]; then
  node_bin=$(command -v node || true)
fi
if [[ -z "$node_bin" ]]; then
  echo "node executable not found" >&2
  exit 1
fi

uv_bin=${UV_BIN:-}
if [[ -z "$uv_bin" ]]; then
  uv_bin=$(command -v uv || true)
fi
if [[ -z "$uv_bin" && -x "$HOME/.local/bin/uv" ]]; then
  uv_bin="$HOME/.local/bin/uv"
fi
if [[ -z "$uv_bin" ]]; then
  echo "uv executable not found" >&2
  exit 1
fi

echo "::group::frontend navigation and direct-route gates"
"$node_bin" --test apps/web/test/tenant-features.test.ts
echo "::endgroup::"

echo "::group::Python AI live-entitlement outage gates"
"$uv_bin" run --project apps/ai-recommendation-service pytest apps/ai-recommendation-service/tests/test_feature_flags.py -q
"$uv_bin" run --project apps/ai-prediction-service pytest apps/ai-prediction-service/test_ai_prediction/test_feature_flags.py -q
"$uv_bin" run --project apps/career-guidance-service pytest apps/career-guidance-service/test_career_guidance/test_feature_flags.py -q
echo "::endgroup::"

echo "::group::gateway API gate and two-tenant matrix"
(cd apps/api-gateway && GOWORK=off "$go_bin" test ./internal/gateway -run 'FeatureFlag' -count=1)
echo "::endgroup::"

echo "::group::background job gate and two-tenant matrix"
(cd apps/report-service && GOWORK=off "$go_bin" test ./tests/unit -run 'Materialize.*Feature' -count=1)
echo "::endgroup::"

echo "::group::event-consumer disabled-feature skip"
(cd apps/notification-service && GOWORK=off "$go_bin" test ./tests/unit -run 'HandleCloudEvent_FlagDisabledSkips' -count=1)
echo "::endgroup::"

echo "Feature-flag matrix passed: frontend hidden, direct route denied, API denied, background job skipped, event ignored, tenant enablement isolated, and Go/Python production outages fail closed."
