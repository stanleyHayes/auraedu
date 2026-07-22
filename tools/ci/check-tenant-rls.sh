#!/usr/bin/env bash
set -euo pipefail

# Every table declared with a tenant_id column must have database-enforced
# isolation somewhere in that service's complete migration history. This is a
# structural coverage gate; service integration tests still prove behavior.

scan_tmp="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-rls.XXXXXX")"
trap 'rm -rf "$scan_tmp"' EXIT
tenant_tables="$scan_tmp/tenant-tables"

for directory in apps/*-service/migrations apps/identity-service/internal/db/migrations; do
  [[ -d "$directory" ]] || continue
  awk -v service="$directory" '
    BEGIN { table = ""; block = "" }
    /^[[:space:]]*CREATE TABLE/ {
      line = $0
      sub(/^[[:space:]]*CREATE TABLE([[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS)?[[:space:]]+/, "", line)
      split(line, parts, /[[:space:]\(]/)
      table = parts[1]
      gsub(/"/, "", table)
      block = $0
      if ($0 ~ /\);/) {
        if (block !~ /auraedu: rls-exempt/ && (block ~ /tenant_(id|code)[[:space:]]/ || (service ~ /tenant-service\/migrations$/ && table == "tenants"))) print service "|" table
        table = ""
        block = ""
      }
      next
    }
    table != "" {
      block = block "\n" $0
      if ($0 ~ /\);/) {
        if (block !~ /auraedu: rls-exempt/ && (block ~ /tenant_(id|code)[[:space:]]/ || (service ~ /tenant-service\/migrations$/ && table == "tenants"))) print service "|" table
        table = ""
        block = ""
      }
    }
  ' "$directory"/*.sql
done | sort -u >"$tenant_tables"

if [[ -n "${AURA_RLS_INVENTORY_FILE:-}" ]]; then
  cp "$tenant_tables" "$AURA_RLS_INVENTORY_FILE"
fi

failed=0
checked=0
while IFS='|' read -r directory table; do
  [[ -n "$directory" && -n "$table" ]] || continue
  checked=$((checked + 1))
  flattened="$scan_tmp/sql"
  cat "$directory"/*.sql | tr '\n' ' ' >"$flattened"

  missing=()
  grep -Eiq "ALTER[[:space:]]+TABLE[[:space:]]+$table[[:space:]]+ENABLE[[:space:]]+ROW[[:space:]]+LEVEL[[:space:]]+SECURITY" "$flattened" || missing+=("ENABLE RLS")
  grep -Eiq "ALTER[[:space:]]+TABLE[[:space:]]+$table[[:space:]]+FORCE[[:space:]]+ROW[[:space:]]+LEVEL[[:space:]]+SECURITY" "$flattened" || missing+=("FORCE RLS")
  grep -Eiq "CREATE[[:space:]]+POLICY[^;]+ON[[:space:]]+$table([[:space:]]|$)[^;]+current_setting\('app\.tenant_id'" "$flattened" || missing+=("tenant policy")

  if (( ${#missing[@]} > 0 )); then
    printf 'ERROR: %s table %s is missing: %s\n' "$directory" "$table" "$(IFS=', '; echo "${missing[*]}")" >&2
    failed=1
  fi
done <"$tenant_tables"

if [[ "$failed" != "0" ]]; then
  exit 1
fi

echo "Tenant RLS coverage passed: $checked tenant-owned table declarations enable and force RLS with a tenant-context policy."
