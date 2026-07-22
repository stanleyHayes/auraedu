#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-growth-smoke.XXXXXX")"
FEATURES_REGISTRY="$ROOT_DIR/tools/smoke/fixtures/growth-features.yaml"
NATS_URL="${NATS_URL:-nats://127.0.0.1:4222}"
POSTGRES_BASE="${POSTGRES_BASE:-postgres://auraedu:auraedu@127.0.0.1:5432}"
CRM_PORT="${CRM_PORT:-18105}"
ANALYTICS_PORT="${ANALYTICS_PORT:-18102}"
NOTIFICATION_PORT="${NOTIFICATION_PORT:-18099}"
AUDIT_PORT="${AUDIT_PORT:-18104}"
TENANT_ID="${TENANT_ID:-upshs}"
SERVICE_TOKEN="growth-smoke-service-token"
PIDS=()

cleanup() {
  local pid
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait "${PIDS[@]:-}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

fail() {
  printf 'Growth smoke failed: %s\n' "$1" >&2
  printf 'Service logs: %s\n' "$RUN_DIR" >&2
  for log in "$RUN_DIR"/*.log; do
    if [[ -f "$log" ]]; then
      printf '\n--- %s ---\n' "$(basename "$log")" >&2
      tail -n 80 "$log" >&2
    fi
  done
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

wait_http() {
  local url="$1"
  local attempts=60
  until curl --fail --silent --show-error "$url" >/dev/null 2>&1; do
    attempts=$((attempts - 1))
    [[ "$attempts" -gt 0 ]] || fail "service did not become ready: $url"
    sleep 1
  done
}

poll_json() {
  local url="$1"
  local expression="$2"
  local output="$3"
  local attempts=40
  until curl --fail-with-body --silent --show-error \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "X-Actor-User: smoke-admin" \
    -H "X-Actor-Tenant: $TENANT_ID" \
    -H "X-Actor-Role: tenant_admin" \
    -H "X-Actor-Permissions: crm.lead.read,analytics.view,notifications.read,audit.read" \
    "$url" >"$output" && jq -e "$expression" "$output" >/dev/null; do
    attempts=$((attempts - 1))
    [[ "$attempts" -gt 0 ]] || fail "eventual assertion failed for $url: $expression"
    sleep 1
  done
}

start_service() {
  local service="$1"
  local mode="$2"
  local port="$3"
  local database="$4"
  local binary="$RUN_DIR/$service"
  local log="$RUN_DIR/$service-$mode.log"
  (
    cd "$ROOT_DIR/apps/$service-service"
    env \
      PORT="$port" \
      DATABASE_URL="$POSTGRES_BASE/$database?sslmode=disable" \
      NATS_URL="$NATS_URL" \
      FEATURES_REGISTRY="$FEATURES_REGISTRY" \
      INTERNAL_SERVICE_TOKEN="$SERVICE_TOKEN" \
      SERVICE_CRM_URL="http://127.0.0.1:$CRM_PORT" \
      "$binary" "$mode"
  ) >"$log" 2>&1 &
  PIDS+=("$!")
}

ensure_database() {
  local database="$1"
  local compose=(docker compose -f "$ROOT_DIR/deploy/docker-compose.infra.yml")
  if ! "${compose[@]}" exec -T postgres psql -U auraedu -d postgres -tAc \
    "SELECT 1 FROM pg_database WHERE datname = '$database'" | grep -qx '1'; then
    "${compose[@]}" exec -T postgres createdb -U auraedu "$database"
  fi
}

migrate_service() {
  local service="$1"
  local database="$2"
  (
    cd "$ROOT_DIR/apps/$service-service"
    env DATABASE_URL="$POSTGRES_BASE/$database?sslmode=disable" \
      "$RUN_DIR/$service" migrate
  ) >"$RUN_DIR/$service-migrate.log" 2>&1
}

need curl
need docker
need go
need jq

for service in crm analytics notification audit; do
  (
    cd "$ROOT_DIR/apps/$service-service"
    GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/auraedu-go-cache}" go build -o "$RUN_DIR/$service" "./cmd/$service-service"
  )
done

# Existing local volumes can predate newly introduced services. Reconcile the
# logical DBs without deleting data, then run each migration ledger exactly
# once before server/worker pairs start concurrently.
for database in crm analytics notification audit; do
  ensure_database "$database"
done
for service in crm analytics notification audit; do
  migrate_service "$service" "$service" || fail "$service migrations failed"
done

start_service crm server "$CRM_PORT" crm
start_service analytics server "$ANALYTICS_PORT" analytics
start_service analytics worker "$ANALYTICS_PORT" analytics
start_service notification server "$NOTIFICATION_PORT" notification
start_service notification worker "$NOTIFICATION_PORT" notification
start_service audit server "$AUDIT_PORT" audit
start_service audit worker "$AUDIT_PORT" audit

wait_http "http://127.0.0.1:$CRM_PORT/ready"
wait_http "http://127.0.0.1:$ANALYTICS_PORT/ready"
wait_http "http://127.0.0.1:$NOTIFICATION_PORT/ready"
wait_http "http://127.0.0.1:$AUDIT_PORT/ready"

# Give durable subscriptions time to register before publishing the capture.
sleep 1

CAPTURE_KEY="growth-smoke-capture-$(date +%s)-$$"
CAPTURE_EMAIL="ama.growth-smoke.$(date +%s).$$@example.com"
CAPTURE_RESPONSE="$RUN_DIR/capture.json"
curl --fail-with-body --silent --show-error \
  -X POST "http://127.0.0.1:$CRM_PORT/api/v1/public/leads" \
  -H 'Content-Type: application/json' \
  -H "X-Tenant-Code: $TENANT_ID" \
  -H "Idempotency-Key: $CAPTURE_KEY" \
  --data "{\"first_name\":\"Ama\",\"last_name\":\"Mensah\",\"email\":\"$CAPTURE_EMAIL\",\"source\":\"marketing_website\",\"message\":\"I would like to learn more about admissions.\",\"preferred_programme_ids\":[],\"consent\":{\"privacy_notice_version\":\"2026-07-18\",\"email\":true,\"sms\":false,\"whatsapp\":false,\"voice\":false}}" \
  >"$CAPTURE_RESPONSE"

LEAD_ID="$(jq -er '.lead_id' "$CAPTURE_RESPONSE")" || fail "lead capture did not return lead_id"
jq -e '.created == true and .stage == "new"' "$CAPTURE_RESPONSE" >/dev/null || fail "lead capture response was not newly created"

poll_json \
  "http://127.0.0.1:$CRM_PORT/api/v1/leads?limit=100" \
  ".data | any(.id == \"$LEAD_ID\" and .tenant_id == \"$TENANT_ID\")" \
  "$RUN_DIR/leads.json"

poll_json \
  "http://127.0.0.1:$ANALYTICS_PORT/api/v1/analytics/metrics?metric_name=growth.leads.count&limit=100" \
  '.data | any(.metric_name == "growth.leads.count" and .value >= 1)' \
  "$RUN_DIR/analytics.json"

poll_json \
  "http://127.0.0.1:$NOTIFICATION_PORT/api/v1/messages?recipient_id=$LEAD_ID&limit=100" \
  ".data | any(.recipient_id == \"$LEAD_ID\" and .channel == \"email\" and .status == \"sent\" and .metadata.consent_verified == true)" \
  "$RUN_DIR/notification.json"

poll_json \
  "http://127.0.0.1:$AUDIT_PORT/api/v1/audit-logs?limit=100" \
  ".data | any(.tenant_id == \"$TENANT_ID\" and .event_type == \"lead.created\" and .resource_id == \"$LEAD_ID\")" \
  "$RUN_DIR/audit.json"

FEEDBACK_KEY="growth-smoke-feedback-$(date +%s)-$$"
curl --fail-with-body --silent --show-error \
  -X POST "http://127.0.0.1:$CRM_PORT/api/v1/public/feedback" \
  -H 'Content-Type: application/json' \
  -H "X-Tenant-Code: $TENANT_ID" \
  -H "Idempotency-Key: $FEEDBACK_KEY" \
  --data '{"feedback_type":"helpful","rating":5,"comment":"The admissions information was clear."}' \
  >/dev/null

if PREFERRED_AT="$(date -u -d '+2 hours' '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null)"; then
  :
else
  PREFERRED_AT="$(date -u -v+2H '+%Y-%m-%dT%H:%M:%SZ')"
fi
CALLBACK_KEY="growth-smoke-callback-$(date +%s)-$$"
CALLBACK_RESPONSE="$RUN_DIR/callback.json"
CALLBACK_PAYLOAD="{\"first_name\":\"Esi\",\"last_name\":\"Quaye\",\"email\":null,\"phone\":\"+233240000099\",\"preferred_at\":\"$PREFERRED_AT\",\"timezone\":\"Africa/Accra\",\"locale\":\"en-GH\",\"message\":\"Please call me about admissions.\",\"consent\":{\"privacy_notice_version\":\"2026-07-19\",\"email\":false,\"sms\":false,\"whatsapp\":false,\"voice\":true}}"
CALLBACK_STATUS="$(curl --silent --show-error \
  -o "$CALLBACK_RESPONSE" -w '%{http_code}' \
  -X POST "http://127.0.0.1:$CRM_PORT/api/v1/public/callback-requests" \
  -H 'Content-Type: application/json' \
  -H "X-Tenant-Code: $TENANT_ID" \
  -H "Idempotency-Key: $CALLBACK_KEY" \
  --data "$CALLBACK_PAYLOAD")"
[[ "$CALLBACK_STATUS" == "201" ]] || fail "callback request returned HTTP $CALLBACK_STATUS"
CALLBACK_ID="$(jq -er '.id' "$CALLBACK_RESPONSE")" || fail "callback request did not return id"
CALLBACK_LEAD_ID="$(jq -er '.lead_id' "$CALLBACK_RESPONSE")" || fail "callback request did not return lead_id"
jq -e ".status == \"requested\" and .preferred_at == \"$PREFERRED_AT\" and .timezone == \"Africa/Accra\"" "$CALLBACK_RESPONSE" >/dev/null || fail "callback response did not preserve the requested time"

CALLBACK_REPLAY="$RUN_DIR/callback-replay.json"
CALLBACK_REPLAY_STATUS="$(curl --silent --show-error \
  -o "$CALLBACK_REPLAY" -w '%{http_code}' \
  -X POST "http://127.0.0.1:$CRM_PORT/api/v1/public/callback-requests" \
  -H 'Content-Type: application/json' \
  -H "X-Tenant-Code: $TENANT_ID" \
  -H "Idempotency-Key: $CALLBACK_KEY" \
  --data "$CALLBACK_PAYLOAD")"
[[ "$CALLBACK_REPLAY_STATUS" == "200" ]] || fail "callback replay returned HTTP $CALLBACK_REPLAY_STATUS"
jq -e ".id == \"$CALLBACK_ID\" and .lead_id == \"$CALLBACK_LEAD_ID\"" "$CALLBACK_REPLAY" >/dev/null || fail "callback replay did not return the original resource"

poll_json \
  "http://127.0.0.1:$CRM_PORT/api/v1/callback-requests?status=requested&limit=100" \
  ".data | any(.id == \"$CALLBACK_ID\" and .lead_id == \"$CALLBACK_LEAD_ID\")" \
  "$RUN_DIR/callbacks.json"

poll_json \
  "http://127.0.0.1:$AUDIT_PORT/api/v1/audit-logs?limit=100" \
  ".data | any(.tenant_id == \"$TENANT_ID\" and .event_type == \"growth.callback_requested\" and .resource_id == \"$CALLBACK_ID\")" \
  "$RUN_DIR/callback-audit.json"

printf 'Growth vertical slice passed.\n'
printf '  tenant: %s\n' "$TENANT_ID"
printf '  lead: %s\n' "$LEAD_ID"
printf '  callback request: %s\n' "$CALLBACK_ID"
printf '  verified: CRM capture/read, analytics projection, consented welcome notification, audit sink, feedback intake, callback request/replay/staff queue\n'
