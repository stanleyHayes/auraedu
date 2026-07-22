#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-assistant-smoke.XXXXXX")"
GO_BIN="${GO_BIN:-/opt/homebrew/bin/go}"
TENANT_ID="${TENANT_ID:-upshs}"
KNOWLEDGE_PORT="${KNOWLEDGE_PORT:-18110}"
ASSISTANT_PORT="${ASSISTANT_PORT:-18111}"
SERVICE_TOKEN="assistant-smoke-service-token"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-auraedu-infra}"
NONCE="$(date +%s)-$$"
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
  printf 'Knowledge assistant smoke failed: %s\n' "$1" >&2
  for log in "$RUN_DIR"/*.log; do
    if [[ -f "$log" ]]; then
      printf '\n%s\n' "$(basename "$log")" >&2
      tail -n 80 "$log" >&2
    fi
  done
  exit 1
}

wait_http() {
  local url="$1" attempts=60
  until curl --fail --silent "$url" >/dev/null 2>&1; do
    attempts=$((attempts - 1))
    [[ "$attempts" -gt 0 ]] || fail "service did not become ready: $url"
    sleep 1
  done
}

command -v docker >/dev/null || fail "docker is required"
command -v curl >/dev/null || fail "curl is required"
command -v jq >/dev/null || fail "jq is required"
[[ -x "$GO_BIN" ]] || fail "Go toolchain not found at $GO_BIN"

cd "$ROOT_DIR"
COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose -f deploy/docker-compose.yml up -d postgres nats >/dev/null

COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME" docker compose -f deploy/docker-compose.yml exec -T postgres psql -U auraedu -d postgres -v ON_ERROR_STOP=1 <<'SQL' >/dev/null
SELECT 'CREATE DATABASE knowledge' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'knowledge')\gexec
SELECT 'CREATE DATABASE assistant' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'assistant')\gexec
SQL

FEATURES_REGISTRY="$ROOT_DIR/tools/smoke/fixtures/growth-features.yaml" \
DATABASE_URL="postgres://auraedu:auraedu@127.0.0.1:5432/knowledge?sslmode=disable" \
MIGRATIONS_PATH="$ROOT_DIR/apps/knowledge-service/migrations" \
NATS_URL="nats://127.0.0.1:4222" INTERNAL_SERVICE_TOKEN="$SERVICE_TOKEN" PORT="$KNOWLEDGE_PORT" \
GOCACHE="${TMPDIR:-/tmp}/auraedu-go-cache" "$GO_BIN" run ./apps/knowledge-service/cmd/knowledge-service server >"$RUN_DIR/knowledge.log" 2>&1 &
PIDS+=("$!")
wait_http "http://127.0.0.1:$KNOWLEDGE_PORT/health"

FEATURES_REGISTRY="$ROOT_DIR/tools/smoke/fixtures/growth-features.yaml" \
DATABASE_URL="postgres://auraedu:auraedu@127.0.0.1:5432/assistant?sslmode=disable" \
MIGRATIONS_PATH="$ROOT_DIR/apps/ai-orchestrator-service/migrations" \
NATS_URL="nats://127.0.0.1:4222" INTERNAL_SERVICE_TOKEN="$SERVICE_TOKEN" \
SERVICE_KNOWLEDGE_URL="http://127.0.0.1:$KNOWLEDGE_PORT" PORT="$ASSISTANT_PORT" \
GOCACHE="${TMPDIR:-/tmp}/auraedu-go-cache" "$GO_BIN" run ./apps/ai-orchestrator-service/cmd/ai-orchestrator-service server >"$RUN_DIR/assistant.log" 2>&1 &
PIDS+=("$!")
wait_http "http://127.0.0.1:$ASSISTANT_PORT/health"

EFFECTIVE_AT="$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)"
SOURCE_RESPONSE="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$KNOWLEDGE_PORT/api/v1/knowledge/sources" \
  -H 'Content-Type: application/json' -H "X-Actor-User: smoke-manager" -H "X-Actor-Tenant: $TENANT_ID" \
  -H 'X-Actor-Permissions: knowledge.manage,knowledge.read' \
  --data "{\"source_type\":\"programme\",\"title\":\"QuantumRiver ${NONCE} programme guide\",\"owner\":\"Admissions\",\"content\":\"The QuantumRiver ${NONCE} programme accepts applications through the official applicant portal and the verified application fee is GHS 250.\",\"effective_at\":\"${EFFECTIVE_AT}\",\"confidentiality\":\"public\",\"locale\":\"en-GH\"}")"
SOURCE_ID="$(jq -r '.id' <<<"$SOURCE_RESPONSE")"
[[ "$SOURCE_ID" != "null" && -n "$SOURCE_ID" ]] || fail "source was not created"

curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$KNOWLEDGE_PORT/api/v1/knowledge/sources/$SOURCE_ID/approve" \
  -H 'Content-Type: application/json' -H "X-Actor-User: smoke-reviewer" -H "X-Actor-Tenant: $TENANT_ID" \
  -H 'X-Actor-Permissions: knowledge.approve' --data '{"review_note":"Verified for live assistant smoke"}' >/dev/null

IDEMPOTENCY_KEY="assistant-smoke-${NONCE}"
ASK_BODY="{\"question\":\"How do I apply for QuantumRiver ${NONCE}?\",\"session_id\":null,\"locale\":\"en-GH\"}"
ANSWER="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$ASSISTANT_PORT/api/v1/public/assistant/messages" \
  -H 'Content-Type: application/json' -H "X-Tenant-Code: $TENANT_ID" -H "Idempotency-Key: $IDEMPOTENCY_KEY" --data "$ASK_BODY")"
jq -e --arg source "$SOURCE_ID" '.answer | contains("QuantumRiver")' <<<"$ANSWER" >/dev/null || fail "answer was not grounded in approved passage"
jq -e --arg source "$SOURCE_ID" '.citations | length == 1 and .[0].source_id == $source' <<<"$ANSWER" >/dev/null || fail "answer did not cite approved source"
jq -e '.locale == "en-GH"' <<<"$ANSWER" >/dev/null || fail "answer did not preserve the requested English locale"
MESSAGE_ID="$(jq -r '.message_id' <<<"$ANSWER")"

REPLAY="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$ASSISTANT_PORT/api/v1/public/assistant/messages" \
  -H 'Content-Type: application/json' -H "X-Tenant-Code: $TENANT_ID" -H "Idempotency-Key: $IDEMPOTENCY_KEY" --data "$ASK_BODY")"
[[ "$(jq -r '.message_id' <<<"$REPLAY")" == "$MESSAGE_ID" ]] || fail "idempotent replay created a second response"

UNSUPPORTED="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$ASSISTANT_PORT/api/v1/public/assistant/messages" \
  -H 'Content-Type: application/json' -H "X-Tenant-Code: $TENANT_ID" -H "Idempotency-Key: assistant-unsupported-${NONCE}" \
  --data '{"question":"Do you guarantee ZzyzxNeverDocumented admission?","session_id":null,"locale":"en"}')"
jq -e '.needs_human == true and .confidence == 0 and (.citations | length == 0)' <<<"$UNSUPPORTED" >/dev/null || fail "unsupported question did not fail closed"

FRENCH_SOURCE_RESPONSE="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$KNOWLEDGE_PORT/api/v1/knowledge/sources" \
  -H 'Content-Type: application/json' -H "X-Actor-User: smoke-manager" -H "X-Actor-Tenant: $TENANT_ID" \
  -H 'X-Actor-Permissions: knowledge.manage,knowledge.read' \
  --data "{\"source_type\":\"programme\",\"title\":\"Guide RivièreQuantique ${NONCE}\",\"owner\":\"Admissions\",\"content\":\"Le programme RivièreQuantique ${NONCE} accepte les candidatures sur le portail officiel et les frais vérifiés sont de 250 GHS.\",\"effective_at\":\"${EFFECTIVE_AT}\",\"confidentiality\":\"public\",\"locale\":\"fr-GH\"}")"
FRENCH_SOURCE_ID="$(jq -r '.id' <<<"$FRENCH_SOURCE_RESPONSE")"
[[ "$FRENCH_SOURCE_ID" != "null" && -n "$FRENCH_SOURCE_ID" ]] || fail "French source was not created"
curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$KNOWLEDGE_PORT/api/v1/knowledge/sources/$FRENCH_SOURCE_ID/approve" \
  -H 'Content-Type: application/json' -H "X-Actor-User: smoke-reviewer" -H "X-Actor-Tenant: $TENANT_ID" \
  -H 'X-Actor-Permissions: knowledge.approve' --data '{"review_note":"Version française vérifiée"}' >/dev/null

FRENCH_ANSWER="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$ASSISTANT_PORT/api/v1/public/assistant/messages" \
  -H 'Content-Type: application/json' -H "X-Tenant-Code: $TENANT_ID" -H "Idempotency-Key: assistant-french-${NONCE}" \
  --data "{\"question\":\"Comment candidater au programme RivièreQuantique ${NONCE} ?\",\"session_id\":null,\"locale\":\"fr-GH\"}")"
jq -e --arg source "$FRENCH_SOURCE_ID" '.locale == "fr-GH" and (.answer | contains("RivièreQuantique")) and (.citations | length == 1 and .[0].source_id == $source)' <<<"$FRENCH_ANSWER" >/dev/null || fail "French answer was not grounded in the French source"

CROSS_LANGUAGE="$(curl --fail-with-body --silent --show-error -X POST "http://127.0.0.1:$ASSISTANT_PORT/api/v1/public/assistant/messages" \
  -H 'Content-Type: application/json' -H "X-Tenant-Code: $TENANT_ID" -H "Idempotency-Key: assistant-cross-language-${NONCE}" \
  --data '{"question":"Tell me about RivièreQuantique","session_id":null,"locale":"en-GH"}')"
jq -e '.needs_human == true and (.citations | length == 0)' <<<"$CROSS_LANGUAGE" >/dev/null || fail "French source leaked into English retrieval"

printf 'Knowledge assistant smoke passed: tenant=%s English-source=%s French-source=%s message=%s\n' "$TENANT_ID" "$SOURCE_ID" "$FRENCH_SOURCE_ID" "$MESSAGE_ID"
