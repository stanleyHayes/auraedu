#!/usr/bin/env bash
set -euo pipefail

# Local, destructive-only-to-ephemeral-containers recovery rehearsal. This does
# not connect to Render or accept a production DATABASE_URL by design.

readonly POSTGRES_IMAGE="postgres:18-alpine@sha256:9a8afca54e7861fd90fab5fdf4c42477a6b1cb7d293595148e674e0a3181de15"
readonly DRILL_ID="auraedu-dr-$PPID-$$"
readonly SOURCE_CONTAINER="${DRILL_ID}-source"
readonly TARGET_CONTAINER="${DRILL_ID}-target"
readonly POSTGRES_PASSWORD="local-recovery-drill-only"
readonly POSTGRES_DB="auraedu_drill"

drill_dir="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-dr.XXXXXX")"
backup_path="$drill_dir/auraedu.dump"
started_at="$(date +%s)"

cleanup() {
  docker rm -f "$SOURCE_CONTAINER" "$TARGET_CONTAINER" >/dev/null 2>&1 || true
  rm -rf "$drill_dir"
}
trap cleanup EXIT INT TERM

wait_for_postgres() {
  local container="$1"
  local attempt
  for attempt in $(seq 1 60); do
    if docker exec "$container" psql -U postgres -d "$POSTGRES_DB" -c 'SELECT 1' >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "ERROR: PostgreSQL container $container did not become ready." >&2
  docker logs "$container" >&2 || true
  return 1
}

start_database() {
  local container="$1"
  docker run --detach --name "$container" \
    --env "POSTGRES_PASSWORD=$POSTGRES_PASSWORD" \
    --env "POSTGRES_DB=$POSTGRES_DB" \
    "$POSTGRES_IMAGE" >/dev/null
  wait_for_postgres "$container"
}

echo "[1/6] Starting isolated source database"
start_database "$SOURCE_CONTAINER"

echo "[2/6] Seeding tenant-scoped recovery fixtures"
docker exec --interactive "$SOURCE_CONTAINER" psql -v ON_ERROR_STOP=1 -U postgres -d "$POSTGRES_DB" <<'SQL' >/dev/null
CREATE TABLE recovery_fixture (
  tenant_code text NOT NULL,
  record_id integer NOT NULL,
  payload text NOT NULL,
  PRIMARY KEY (tenant_code, record_id)
);
INSERT INTO recovery_fixture (tenant_code, record_id, payload) VALUES
  ('accra-academy', 1, 'sentinel-alpha'),
  ('accra-academy', 2, 'sentinel-beta'),
  ('kumasi-learning', 1, 'sentinel-gamma');
SQL

source_fingerprint="$(docker exec "$SOURCE_CONTAINER" psql -At -U postgres -d "$POSTGRES_DB" -c "SELECT count(*) || ':' || md5(string_agg(tenant_code || ':' || record_id || ':' || payload, '|' ORDER BY tenant_code, record_id)) FROM recovery_fixture;")"

echo "[3/6] Creating and structurally validating custom-format backup"
docker exec "$SOURCE_CONTAINER" pg_dump -U postgres -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges >"$backup_path"
test -s "$backup_path"
docker cp "$backup_path" "$SOURCE_CONTAINER:/tmp/auraedu.dump" >/dev/null
docker exec "$SOURCE_CONTAINER" pg_restore --list /tmp/auraedu.dump >/dev/null

echo "[4/6] Starting isolated recovery target"
start_database "$TARGET_CONTAINER"
docker cp "$backup_path" "$TARGET_CONTAINER:/tmp/auraedu.dump" >/dev/null

echo "[5/6] Restoring with fail-fast ownership-neutral settings"
docker exec "$TARGET_CONTAINER" pg_restore \
  -U postgres \
  -d "$POSTGRES_DB" \
  --no-owner \
  --no-privileges \
  --exit-on-error \
  /tmp/auraedu.dump >/dev/null

echo "[6/6] Verifying record count and deterministic content fingerprint"
target_fingerprint="$(docker exec "$TARGET_CONTAINER" psql -At -U postgres -d "$POSTGRES_DB" -c "SELECT count(*) || ':' || md5(string_agg(tenant_code || ':' || record_id || ':' || payload, '|' ORDER BY tenant_code, record_id)) FROM recovery_fixture;")"

if [[ "$source_fingerprint" != "$target_fingerprint" ]]; then
  echo "ERROR: restored data fingerprint differs from source ($source_fingerprint != $target_fingerprint)." >&2
  exit 1
fi

if ! docker exec "$TARGET_CONTAINER" psql -At -U postgres -d "$POSTGRES_DB" -c "SELECT payload FROM recovery_fixture WHERE tenant_code = 'accra-academy' AND record_id = 1;" | grep -Fxq 'sentinel-alpha'; then
  echo "ERROR: recovery sentinel was not restored." >&2
  exit 1
fi

completed_at="$(date +%s)"
elapsed_seconds="$((completed_at - started_at))"
backup_bytes="$(wc -c <"$backup_path" | tr -d ' ')"

printf '{"result":"pass","postgres_major":18,"records":3,"fingerprint":"%s","backup_bytes":%s,"elapsed_seconds":%s}\n' \
  "$target_fingerprint" "$backup_bytes" "$elapsed_seconds"
