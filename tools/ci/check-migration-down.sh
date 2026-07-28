#!/usr/bin/env bash
# Down-test one service's goose migration chain against a fresh PostgreSQL:
# apply the full chain, roll back the last two migrations, then re-apply them.
# Proves the -- +goose Down sections actually run (agent_plan EP-07 rail).
#
# Usage: bash tools/ci/check-migration-down.sh <service> <postgres-dsn>
# Example: bash tools/ci/check-migration-down.sh identity-service \
#   "postgres://auraedu:auraedu@127.0.0.1:5432/identity?sslmode=disable"
set -euo pipefail

service="${1:?usage: check-migration-down.sh <service> <postgres-dsn>}"
dsn="${2:?usage: check-migration-down.sh <service> <postgres-dsn>}"
migrations_dir="apps/${service}/migrations"

if [[ ! -d "$migrations_dir" ]]; then
  echo "ERROR: no migrations directory for ${service}" >&2
  exit 1
fi

stage="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-migration-down.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
staged="$stage/migrations"
mkdir -p "$staged"
cp "$migrations_dir"/*.sql "$staged/"

# Goose requires a leading '-- +goose Up' annotation; legacy baseline files
# (for example identity-service 0001_init.sql) predate the markers and run
# forward-only in the service runner, so annotate the copies — never the
# append-only originals.
while IFS= read -r file; do
  if ! grep -q -- '-- +goose Up' "$file"; then
    annotated="$stage/annotated.sql"
    { printf -- '-- +goose Up\n\n'; cat "$file"; } >"$annotated"
    mv "$annotated" "$file"
  fi
  if ! grep -q -- '-- +goose Down' "$file"; then
    printf -- '\n-- +goose Down\n-- Intentionally empty: forward-only legacy migration.\n' >>"$file"
  fi
  # Goose splits statements on semicolons unless '-- +goose StatementBegin/End'
  # guards a dollar-quoted body; service runners exec the Up section as one
  # unit, so wrap staged copies that contain $$ bodies without the guard.
  if grep -q '\$\$' "$file" && ! grep -q -- '-- +goose StatementBegin' "$file"; then
    wrapped="$stage/wrapped.sql"
    awk '
      /^-- \+goose Up/ { print; print "-- +goose StatementBegin"; next }
      /^-- \+goose Down/ { print "-- +goose StatementEnd"; print; next }
      { print }
    ' "$file" >"$wrapped"
    mv "$wrapped" "$file"
  fi
done < <(find "$staged" -maxdepth 1 -type f -name '*.sql' | sort)

total="$(find "$staged" -maxdepth 1 -type f -name '*.sql' | wc -l | tr -d ' ')"
if [[ "$total" -lt 2 ]]; then
  echo "ERROR: ${service} needs at least two migrations to down-test" >&2
  exit 1
fi

# goose is pinned to the same v3.27.2 the services compile against; the
# tool@version go run mirrors the pinned govulncheck invocation in go.yml.
goose() {
  GOWORK=off GOTOOLCHAIN=local \
    go run github.com/pressly/goose/v3/cmd/goose@v3.27.2 -dir "$staged" postgres "$dsn" "$@"
}

echo "==> ${service}: applying full migration chain (${total} files)"
goose up

# Roll back the last two migrations, but never cross a migration whose Down
# section carries no SQL (irreversible by design, e.g. audit-service 0003):
# deeper down files may assume that migration's schema, so production rollback
# must stop there too. Re-apply afterwards to prove the path back up.
chain=()
while IFS= read -r path; do
  chain+=("${path##*/}")
done < <(find "$staged" -maxdepth 1 -type f -name '*.sql' | sort -r)
down_has_sql() {
  awk '
    /^-- \+goose Down/ { found = 1; next }
    found && $0 !~ /^[[:space:]]*(--|$)/ { print; exit }
  ' "$staged/$1" | grep -q .
}
rolled_back=0
for migration in "${chain[@]:0:2}"; do
  if ! down_has_sql "$migration"; then
    if [[ "$rolled_back" -eq 0 ]]; then
      goose down
      rolled_back=1
    fi
    echo "WARNING: ${service} ${migration} has an empty Down section (irreversible by design);" >&2
    echo "WARNING: stopping the rollback there — deeper down files cannot assume its schema." >&2
    break
  fi
  goose down
  rolled_back=$((rolled_back + 1))
done
if [[ "$rolled_back" -eq 0 ]]; then
  echo "ERROR: ${service} tip migrations are all irreversible; nothing to down-test" >&2
  exit 1
fi
echo "==> ${service}: rolled back ${rolled_back} migration(s); re-applying"
goose up
echo "==> ${service}: final state"
goose status
echo "Migration down-test passed for ${service}: down files run and re-apply cleanly."
