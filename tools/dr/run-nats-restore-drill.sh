#!/usr/bin/env bash
set -euo pipefail

# Isolated JetStream recovery rehearsal. The source server is destroyed before
# restoration so this cannot silently validate against the original state.

readonly NATS_IMAGE="nats:2.11-alpine@sha256:e4bf19f15fd3218814a4e3c9e0064e1334bd8aa20d5984b9f1a0afd084f8cc00"
readonly NATS_BOX_IMAGE="natsio/nats-box:0.19.2@sha256:8031d190c7ee24081f3f27cc939fb647a1eeb29ebb5c60fef9b5b6c7a846d6a2"
readonly DRILL_ID="auraedu-nats-dr-$PPID-$$"
readonly NETWORK="${DRILL_ID}-network"
readonly SOURCE_CONTAINER="${DRILL_ID}-source"
readonly TARGET_CONTAINER="${DRILL_ID}-target"
readonly SERVER_ALIAS="nats-dr"
readonly STREAM="AURA_DR"
readonly CONSUMER="DR_WORKER"

drill_dir="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-nats-dr.XXXXXX")"
backup_dir="$drill_dir/stream-backup"
started_at="$(date +%s)"

cleanup() {
  docker rm -f "$SOURCE_CONTAINER" "$TARGET_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
  rm -rf "$drill_dir"
}
trap cleanup EXIT INT TERM

wait_for_nats() {
  local container="$1"
  local attempt
  for attempt in $(seq 1 60); do
    if docker exec "$container" wget -qO- http://127.0.0.1:8222/healthz >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "ERROR: NATS container $container did not become ready." >&2
  docker logs "$container" >&2 || true
  return 1
}

start_nats() {
  local container="$1"
  docker run --detach --name "$container" --network "$NETWORK" --network-alias "$SERVER_ALIAS" \
    "$NATS_IMAGE" -js -sd /data -m 8222 >/dev/null
  wait_for_nats "$container"
}

nats_cli() {
  docker run --rm --network "$NETWORK" --volume "$drill_dir:/backup" \
    "$NATS_BOX_IMAGE" nats --server "nats://$SERVER_ALIAS:4222" --no-context "$@"
}

docker network create "$NETWORK" >/dev/null

echo "[1/7] Starting isolated source JetStream"
start_nats "$SOURCE_CONTAINER"

echo "[2/7] Creating file-backed stream, durable consumer and test events"
nats_cli stream add "$STREAM" \
  --subjects 'AURA.DR.>' \
  --storage file \
  --retention limits \
  --replicas 1 \
  --max-msgs 1000 \
  --defaults >/dev/null
nats_cli consumer add "$STREAM" "$CONSUMER" \
  --pull \
  --ack explicit \
  --deliver all \
  --filter 'AURA.DR.>' \
  --defaults >/dev/null

for payload in '{"event_id":"dr-1"}' '{"event_id":"dr-2"}' '{"event_id":"dr-3"}'; do
  nats_cli publish AURA.DR.fixture "$payload" >/dev/null
done

source_state="$(nats_cli stream info "$STREAM" --json)"
source_messages="$(ruby -rjson -e 'puts JSON.parse(STDIN.read).fetch("state").fetch("messages")' <<<"$source_state")"
[[ "$source_messages" == "3" ]] || { echo "ERROR: source stream contains $source_messages messages, expected 3." >&2; exit 1; }
nats_cli consumer info "$STREAM" "$CONSUMER" --json >/dev/null

echo "[3/7] Backing up stream data and durable consumer state"
nats_cli stream backup "$STREAM" /backup/stream-backup --check --consumers --no-progress >/dev/null
backup_probe="$(find "$backup_dir" -type f -size +0c -print -quit)"
[[ -n "$backup_probe" ]] || { echo "ERROR: JetStream backup contains no non-empty files." >&2; exit 1; }

echo "[4/7] Destroying the source broker"
docker rm -f "$SOURCE_CONTAINER" >/dev/null

echo "[5/7] Starting a fresh recovery target"
start_nats "$TARGET_CONTAINER"

echo "[6/7] Restoring the backup into the empty target"
nats_cli stream restore /backup/stream-backup >/dev/null

echo "[7/7] Verifying stream messages, subjects and durable consumer"
target_state="$(nats_cli stream info "$STREAM" --json)"
target_messages="$(ruby -rjson -e 'puts JSON.parse(STDIN.read).fetch("state").fetch("messages")' <<<"$target_state")"
target_subjects="$(ruby -rjson -e 'puts JSON.parse(STDIN.read).fetch("config").fetch("subjects").join(",")' <<<"$target_state")"
consumer_state="$(nats_cli consumer info "$STREAM" "$CONSUMER" --json)"
consumer_name="$(ruby -rjson -e 'puts JSON.parse(STDIN.read).fetch("name")' <<<"$consumer_state")"

[[ "$target_messages" == "3" ]] || { echo "ERROR: restored stream contains $target_messages messages, expected 3." >&2; exit 1; }
[[ "$target_subjects" == "AURA.DR.>" ]] || { echo "ERROR: restored subject is $target_subjects." >&2; exit 1; }
[[ "$consumer_name" == "$CONSUMER" ]] || { echo "ERROR: durable consumer was not restored." >&2; exit 1; }

fingerprint_input=""
for sequence in 1 2 3; do
  message_json="$(nats_cli stream get "$STREAM" "$sequence" --json)"
  message="$(ruby -rjson -rbase64 -e 'document = JSON.parse(STDIN.read); record = document["message"] || document; print Base64.strict_decode64(record.fetch("data"))' <<<"$message_json")"
  fingerprint_input="${fingerprint_input}${message}"
done
fingerprint="$(printf '%s' "$fingerprint_input" | shasum -a 256 | awk '{print $1}')"
expected_fingerprint="$(printf '%s' '{"event_id":"dr-1"}{"event_id":"dr-2"}{"event_id":"dr-3"}' | shasum -a 256 | awk '{print $1}')"
[[ "$fingerprint" == "$expected_fingerprint" ]] || { echo "ERROR: restored message content fingerprint differs from the source fixture." >&2; exit 1; }

completed_at="$(date +%s)"
elapsed_seconds="$((completed_at - started_at))"
backup_bytes="$(du -sk "$backup_dir" | awk '{print $1 * 1024}')"

printf '{"result":"pass","nats_version":"2.11","stream":"%s","consumer":"%s","messages":%s,"fingerprint":"%s","backup_bytes":%s,"elapsed_seconds":%s}\n' \
  "$STREAM" "$CONSUMER" "$target_messages" "$fingerprint" "$backup_bytes" "$elapsed_seconds"
