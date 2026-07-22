#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-campaign-smoke.XXXXXX")"
GO_BIN="${GO_BIN:-/opt/homebrew/bin/go}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-auraedu-infra}"
TENANT_ID="${TENANT_ID:-upshs}"
PORT="${CAMPAIGN_PORT:-18113}"
PIDS=()

cleanup() {
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  wait "${PIDS[@]:-}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

fail() {
  printf 'Campaign lifecycle smoke failed: %s\n' "$1" >&2
  for log in "$RUN_DIR"/*.log; do [[ -f "$log" ]] && tail -n 100 "$log" >&2; done
  exit 1
}

wait_http() {
  local attempts=60
  until curl --fail --silent "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; do
    attempts=$((attempts - 1)); [[ "$attempts" -gt 0 ]] || fail "service did not become ready"; sleep 1
  done
}

[[ -x "$GO_BIN" ]] || fail "Go toolchain not found at $GO_BIN"
command -v docker >/dev/null || fail "docker is required"
command -v curl >/dev/null || fail "curl is required"
command -v jq >/dev/null || fail "jq is required"

cd "$ROOT_DIR"
COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose -f deploy/docker-compose.yml up -d postgres nats >/dev/null
COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose -f deploy/docker-compose.yml exec -T postgres psql -U auraedu -d postgres -v ON_ERROR_STOP=1 <<'SQL' >/dev/null
SELECT 'CREATE DATABASE campaign' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'campaign')\gexec
SQL

COMMON=(
  DATABASE_URL="postgres://auraedu:auraedu@127.0.0.1:5432/campaign?sslmode=disable"
  MIGRATIONS_PATH="$ROOT_DIR/apps/campaign-service/migrations"
  NATS_URL="nats://127.0.0.1:4222"
  GOCACHE="${TMPDIR:-/tmp}/auraedu-go-cache"
)
env "${COMMON[@]}" FEATURES_REGISTRY="$ROOT_DIR/tools/smoke/fixtures/growth-features.yaml" PORT="$PORT" \
  "$GO_BIN" run ./apps/campaign-service/cmd/campaign-service server >"$RUN_DIR/server.log" 2>&1 &
PIDS+=("$!")
wait_http
env "${COMMON[@]}" "$GO_BIN" run ./apps/campaign-service/cmd/campaign-service worker >"$RUN_DIR/worker.log" 2>&1 &
PIDS+=("$!")

START_AT="$(date -u -v+1H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '1 hour' +%Y-%m-%dT%H:%M:%SZ)"
END_AT="$(date -u -v+2d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '2 days' +%Y-%m-%dT%H:%M:%SZ)"
BASE_HEADERS=(-H 'Content-Type: application/json' -H "X-Actor-Tenant: $TENANT_ID")
CREATED="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$PORT/api/v1/campaigns" \
  "${BASE_HEADERS[@]}" -H 'X-Actor-User: smoke-owner' -H 'X-Actor-Permissions: campaign.create,campaign.read,campaign.update' \
  --data "{\"name\":\"Open day smoke $(date +%s)\",\"objective\":\"Generate qualified applications\",\"channel\":\"event\",\"audience_definition\":\"Prospective students and guardians\",\"programme_ids\":[],\"budget\":2500,\"currency\":\"GHS\",\"start_at\":\"$START_AT\",\"end_at\":\"$END_AT\"}")"
CAMPAIGN_ID="$(jq -r '.id' <<<"$CREATED")"
[[ "$CAMPAIGN_ID" != "null" && -n "$CAMPAIGN_ID" ]] || fail "campaign was not created"
jq -e '.status == "draft" and (.tracking_url_parameters | contains("utm_campaign"))' <<<"$CREATED" >/dev/null || fail "draft tracking invariant failed"

curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$PORT/api/v1/campaigns/$CAMPAIGN_ID/submit-for-approval" \
  "${BASE_HEADERS[@]}" -H 'X-Actor-User: smoke-owner' -H 'X-Actor-Permissions: campaign.update' --data '{}' >/dev/null

SELF_CODE="$(curl --silent --output /dev/null --write-out '%{http_code}' -X POST "http://127.0.0.1:$PORT/api/v1/campaigns/$CAMPAIGN_ID/approve" \
  "${BASE_HEADERS[@]}" -H 'X-Actor-User: smoke-owner' -H 'X-Actor-Permissions: campaign.approve,campaign.budget.approve' --data '{"review_note":"self review"}')"
[[ "$SELF_CODE" == "409" ]] || fail "four-eyes self approval returned $SELF_CODE"

APPROVED="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$PORT/api/v1/campaigns/$CAMPAIGN_ID/approve" \
  "${BASE_HEADERS[@]}" -H 'X-Actor-User: smoke-reviewer' -H 'X-Actor-Permissions: campaign.approve,campaign.budget.approve' --data '{"review_note":"Audience, dates and budget verified"}')"
jq -e '.status == "approved" and .approved_by == "smoke-reviewer"' <<<"$APPROVED" >/dev/null || fail "independent approval failed"

PUBLISHED="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$PORT/api/v1/campaigns/$CAMPAIGN_ID/publish" \
  "${BASE_HEADERS[@]}" -H 'X-Actor-User: smoke-publisher' -H 'X-Actor-Permissions: campaign.publish' --data '{}')"
jq -e '.status == "scheduled"' <<<"$PUBLISHED" >/dev/null || fail "future campaign was not scheduled"

attempts=30
while [[ "$(COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose -f deploy/docker-compose.yml exec -T postgres psql -U auraedu -d campaign -tAc "SELECT count(*) FROM campaign_outbox WHERE published_at IS NULL")" != "0" ]]; do
  attempts=$((attempts - 1)); [[ "$attempts" -gt 0 ]] || fail "transactional outbox did not drain"; sleep 1
done

printf 'Campaign lifecycle smoke passed: tenant=%s campaign=%s status=scheduled outbox=drained\n' "$TENANT_ID" "$CAMPAIGN_ID"
