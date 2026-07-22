#!/usr/bin/env bash
# Proves the production onboarding seam with real PostgreSQL, NATS JetStream,
# Tenant and Identity HTTP servers, and the durable Identity event worker.
set -Eeuo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-onboarding-activation.XXXXXX")"
run_id="$$"
postgres_container="auraedu-activation-postgres-${run_id}"
nats_container="auraedu-activation-nats-${run_id}"
redis_container="auraedu-activation-redis-${run_id}"
gateway_port=18180
tenant_port=18082
identity_port=18081
capture_port=18099
internal_token="auraedu-activation-smoke-internal-token"
tenant_pid=""
tenant_worker_pid=""
gateway_pid=""
identity_pid=""
worker_pid=""
capture_pid=""

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command not found: $1" >&2
    exit 1
  }
}

port_available() {
  ! curl --silent --show-error --max-time 1 "http://127.0.0.1:$1/healthz" >/dev/null 2>&1
}

cleanup() {
  exit_code=$?
  trap - EXIT INT TERM
  for pid in "$worker_pid" "$tenant_worker_pid" "$gateway_pid" "$identity_pid" "$tenant_pid" "$capture_pid"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  docker rm -f "$redis_container" "$nats_container" "$postgres_container" >/dev/null 2>&1 || true
  if [[ $exit_code -ne 0 ]]; then
    echo "onboarding activation smoke failed; diagnostics: $run_dir" >&2
    for log_file in gateway tenant tenant-worker identity worker capture; do
      if [[ -f "$run_dir/$log_file.log" ]]; then
        echo "--- $log_file.log" >&2
        tail -80 "$run_dir/$log_file.log" >&2
      fi
    done
  else
    rm -rf "$run_dir"
  fi
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

for command_name in curl docker go jq; do
  require_command "$command_name"
done
for port in "$gateway_port" "$tenant_port" "$identity_port" "$capture_port"; do
  if ! port_available "$port"; then
    echo "port $port is already serving HTTP; stop that process and retry" >&2
    exit 1
  fi
done

echo "[1/8] starting isolated PostgreSQL, Redis, and NATS JetStream"
docker run --detach --name "$postgres_container" \
  --env POSTGRES_USER=auraedu --env POSTGRES_PASSWORD=auraedu \
  --publish 127.0.0.1::5432 postgres:18-alpine@sha256:9a8afca54e7861fd90fab5fdf4c42477a6b1cb7d293595148e674e0a3181de15 >/dev/null
docker run --detach --name "$nats_container" \
  --publish 127.0.0.1::4222 nats:2.11-alpine@sha256:e4bf19f15fd3218814a4e3c9e0064e1334bd8aa20d5984b9f1a0afd084f8cc00 -js >/dev/null
docker run --detach --name "$redis_container" \
  --publish 127.0.0.1::6379 valkey/valkey:8-alpine@sha256:94365b275456ae14621001c03556c732b1d93a0cdeacc317d1bdd52eba680885 >/dev/null

for _ in $(seq 1 60); do
  if docker exec "$postgres_container" pg_isready -U auraedu >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$postgres_container" pg_isready -U auraedu >/dev/null
docker exec "$postgres_container" createdb -U auraedu auraedu_tenant_activation
docker exec "$postgres_container" createdb -U auraedu auraedu_identity_activation

postgres_port="$(docker inspect --format '{{(index (index .NetworkSettings.Ports "5432/tcp") 0).HostPort}}' "$postgres_container")"
nats_port="$(docker inspect --format '{{(index (index .NetworkSettings.Ports "4222/tcp") 0).HostPort}}' "$nats_container")"
redis_port="$(docker inspect --format '{{(index (index .NetworkSettings.Ports "6379/tcp") 0).HostPort}}' "$redis_container")"
tenant_dsn="postgres://auraedu:auraedu@127.0.0.1:${postgres_port}/auraedu_tenant_activation?sslmode=disable"
identity_dsn="postgres://auraedu:auraedu@127.0.0.1:${postgres_port}/auraedu_identity_activation?sslmode=disable"
nats_url="nats://127.0.0.1:${nats_port}"
redis_url="redis://127.0.0.1:${redis_port}"

echo "[2/8] building the real service binaries and notification capture"
(cd "$repo_root/apps/api-gateway" && go build -o "$run_dir/api-gateway" ./cmd/api-gateway)
(cd "$repo_root/apps/tenant-service" && go build -o "$run_dir/tenant-service" ./cmd/tenant-service)
(cd "$repo_root/apps/identity-service" && go build -o "$run_dir/identity-service" ./cmd/identity-service)
go build -o "$run_dir/transactional-email-capture" "$repo_root/tools/smoke/helpers/transactional-email-capture.go"

start_tenant() {
  (
    cd "$repo_root/apps/tenant-service"
    env PORT="$tenant_port" DATABASE_URL="$tenant_dsn" NATS_URL="$nats_url" \
      INTERNAL_SERVICE_TOKEN="$internal_token" "$run_dir/tenant-service" server
  ) >"$run_dir/tenant.log" 2>&1 &
  tenant_pid=$!
}

wait_for_health() {
  local url=$1
  local label=$2
  for _ in $(seq 1 60); do
    if curl --fail --silent --max-time 1 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "$label did not become healthy" >&2
  return 1
}

echo "[3/8] starting Tenant, Identity, the durable worker, and capture service"
INTERNAL_SERVICE_TOKEN="$internal_token" PORT="$capture_port" \
  "$run_dir/transactional-email-capture" >"$run_dir/capture.log" 2>&1 &
capture_pid=$!
start_tenant
# Render may start the server and worker from the same release concurrently.
# The shared database migration advisory lock must make that first boot safe.
(
  cd "$repo_root/apps/tenant-service"
  env DATABASE_URL="$tenant_dsn" NATS_URL="$nats_url" \
    "$run_dir/tenant-service" worker
) >"$run_dir/tenant-worker.log" 2>&1 &
tenant_worker_pid=$!
(
  cd "$repo_root/apps/api-gateway"
  env PORT="$gateway_port" JWT_SIGNING_KEY="activation-smoke-signing-key-with-safe-length" \
    REDIS_URL="$redis_url" RATE_LIMIT_RPS="0.01" RATE_LIMIT_BURST="2" \
    SERVICE_TENANT_URL="http://127.0.0.1:${tenant_port}" \
    "$run_dir/api-gateway" server
) >"$run_dir/gateway.log" 2>&1 &
gateway_pid=$!
(
  cd "$repo_root/apps/identity-service"
  env PORT="$identity_port" DATABASE_URL="$identity_dsn" NATS_URL="$nats_url" \
    JWT_SIGNING_KEY="activation-smoke-signing-key-with-safe-length" \
    SERVICE_TENANT_URL="http://127.0.0.1:${tenant_port}" \
    SERVICE_NOTIFICATION_URL="http://127.0.0.1:${capture_port}" \
    INTERNAL_SERVICE_TOKEN="$internal_token" "$run_dir/identity-service" server
) >"$run_dir/identity.log" 2>&1 &
identity_pid=$!
(
  cd "$repo_root/apps/identity-service"
  env DATABASE_URL="$identity_dsn" NATS_URL="$nats_url" \
    JWT_SIGNING_KEY="activation-smoke-signing-key-with-safe-length" \
    SERVICE_TENANT_URL="http://127.0.0.1:${tenant_port}" \
    SERVICE_NOTIFICATION_URL="http://127.0.0.1:${capture_port}" \
    INTERNAL_SERVICE_TOKEN="$internal_token" "$run_dir/identity-service" worker
) >"$run_dir/worker.log" 2>&1 &
worker_pid=$!

wait_for_health "http://127.0.0.1:${capture_port}/healthz" "notification capture"
wait_for_health "http://127.0.0.1:${tenant_port}/health" "tenant service"
wait_for_health "http://127.0.0.1:${identity_port}/health" "identity service"
wait_for_health "http://127.0.0.1:${gateway_port}/ready" "API gateway"

echo "[4/8] proving gateway replay safety and public abuse protection, then approving"
onboarding_payload='{"school_name":"AuraEDU Activation School","administrator_name":"Ama Mensah","email":"activation-admin@example.edu","country_code":"GH","plan":"growth","privacy_notice_version":"2026-07","accepted_terms":true,"website":""}'
submission="$(curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: auraedu-activation-smoke-2026' \
  --data "$onboarding_payload" \
  "http://127.0.0.1:${gateway_port}/api/v1/public/onboarding-requests")"
request_id="$(jq -er '.request_id' <<<"$submission")"
replayed="$(curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: auraedu-activation-smoke-2026' \
  --data "$onboarding_payload" \
  "http://127.0.0.1:${gateway_port}/api/v1/public/onboarding-requests")"
[[ "$(jq -r '.request_id' <<<"$replayed")" == "$request_id" ]]
abuse_status="$(curl --silent --dump-header "$run_dir/abuse.headers" \
  --output "$run_dir/abuse.json" --write-out '%{http_code}' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: auraedu-activation-smoke-abuse' \
  --data '{"school_name":"Abusive Replay","administrator_name":"Bot User","email":"bot@example.edu","country_code":"GH","plan":"growth","privacy_notice_version":"2026-07","accepted_terms":true,"website":""}' \
  "http://127.0.0.1:${gateway_port}/api/v1/public/onboarding-requests")"
[[ "$abuse_status" == "429" ]]
[[ "$(jq -r '.error.code' "$run_dir/abuse.json")" == "rate_limit_exceeded" ]]
grep -qi '^Retry-After:' "$run_dir/abuse.headers"
approval="$(curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  -H 'X-Actor-User: activation-smoke-platform-admin' \
  -H 'X-Actor-Role: platform_super_admin' \
  --data '{"tenant_code":"activation-school"}' \
  "http://127.0.0.1:${tenant_port}/api/v1/super-admin/onboarding-requests/${request_id}/approve")"
[[ "$(jq -r '.status' <<<"$approval")" == "approved" ]]

echo "[5/8] waiting for the event worker to create and deliver the administrator invite"
delivery=""
for _ in $(seq 1 60); do
  if delivery="$(curl --fail --silent --max-time 1 \
    -H "Authorization: Bearer $internal_token" \
    "http://127.0.0.1:${capture_port}/__capture/latest" 2>/dev/null)"; then
    if [[ "$(jq -r '.template // empty' <<<"$delivery")" == "user_invite" ]]; then
      break
    fi
  fi
  delivery=""
  sleep 1
done
[[ -n "$delivery" ]]
[[ "$(jq -r '.tenant_id' <<<"$delivery")" == "activation-school" ]]
[[ "$(jq -r '.recipient' <<<"$delivery")" == "activation-admin@example.edu" ]]
invite_token="$(jq -er '.data.invite_token' <<<"$delivery")"

tenant_headers=(-H 'X-Actor-User: activation-smoke-platform-admin' -H 'X-Actor-Role: platform_super_admin')
tenant_before="$(curl --fail --silent "${tenant_headers[@]}" "http://127.0.0.1:${tenant_port}/api/v1/tenants/activation-school")"
[[ "$(jq -r '.status' <<<"$tenant_before")" == "onboarding" ]]

unauthorized_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  --request POST "http://127.0.0.1:${tenant_port}/internal/v1/tenants/activation-school/activate")"
[[ "$unauthorized_status" == "401" ]]

echo "[6/8] proving invite acceptance survives a Tenant dependency outage"
kill "$tenant_pid"
wait "$tenant_pid" 2>/dev/null || true
tenant_pid=""
accept_body='{"name":"Ama Mensah","password":"Correct-Horse-Battery-2026!"}'
failed_status="$(curl --silent --output "$run_dir/failed-acceptance.json" --write-out '%{http_code}' \
  -H 'Content-Type: application/json' --data "$accept_body" \
  "http://127.0.0.1:${identity_port}/api/v1/users/invites/${invite_token}/accept")"
[[ "$failed_status" == "503" ]]
[[ "$(jq -r '.code' "$run_dir/failed-acceptance.json")" == "service_unavailable" ]]

echo "[7/8] restarting Tenant and resuming the same one-time invite"
start_tenant
wait_for_health "http://127.0.0.1:${tenant_port}/health" "restarted tenant service"
accepted="$(curl --fail --silent --show-error \
  -H 'Content-Type: application/json' --data "$accept_body" \
  "http://127.0.0.1:${identity_port}/api/v1/users/invites/${invite_token}/accept")"
user_id="$(jq -er '.id' <<<"$accepted")"
[[ "$(jq -r '.tenant_id' <<<"$accepted")" == "activation-school" ]]
[[ "$(jq -r '.role' <<<"$accepted")" == "school_admin" ]]

tenant_after="$(curl --fail --silent "${tenant_headers[@]}" "http://127.0.0.1:${tenant_port}/api/v1/tenants/activation-school")"
[[ "$(jq -r '.status' <<<"$tenant_after")" == "active" ]]

echo "[8/8] proving idempotent retry and usable administrator login"
retried="$(curl --fail --silent --show-error \
  -H 'Content-Type: application/json' --data "$accept_body" \
  "http://127.0.0.1:${identity_port}/api/v1/users/invites/${invite_token}/accept")"
[[ "$(jq -r '.id' <<<"$retried")" == "$user_id" ]]
login="$(curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  --data '{"email":"activation-admin@example.edu","password":"Correct-Horse-Battery-2026!"}' \
  "http://127.0.0.1:${identity_port}/api/v1/auth/login")"
[[ -n "$(jq -er '.access_token' <<<"$login")" ]]
[[ "$(jq -r '.user.id' <<<"$login")" == "$user_id" ]]

echo "PASS: gateway replay/abuse controls, approval invite delivery, outage recovery, idempotent activation, and login all succeeded"
