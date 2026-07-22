#!/usr/bin/env bash
set -euo pipefail

base_ref="${1:-origin/main}"
include_worktree="${AURA_MIGRATIONS_INCLUDE_WORKTREE:-0}"
migration_path_re='^apps/[^/]+/(internal/db/)?migrations/'
migration_file_re='^apps/[^/]+/(internal/db/)?migrations/[0-9]{4}_[a-z0-9_]+\.sql$'

merge_base="$(git merge-base "$base_ref" HEAD)"
check_tmp="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-migrations.XXXXXX")"
trap 'rm -rf "$check_tmp"' EXIT
candidate_file="$check_tmp/candidates"

{
  git diff --name-only "$merge_base"...HEAD
  if [[ "$include_worktree" == "1" ]]; then
    git diff --name-only
    git diff --cached --name-only
    git ls-files --others --exclude-standard
  fi
} | grep -E "$migration_path_re" | sort -u >"$candidate_file" || true

if [[ ! -s "$candidate_file" ]]; then
  echo "No migrations changed."
  exit 0
fi

echo "Changed migrations:"
cat "$candidate_file"

failed=0
while IFS= read -r file; do
  [[ -n "$file" ]] || continue

  if git cat-file -e "$merge_base:$file" 2>/dev/null; then
    if [[ ! -f "$file" ]]; then
      echo "ERROR: Migration files are append-only; deletion or rename is forbidden: $file" >&2
      failed=1
      continue
    fi
    base_hash="$(git rev-parse "$merge_base:$file")"
    current_hash="$(git hash-object "$file")"
    if [[ "$base_hash" != "$current_hash" ]]; then
      echo "ERROR: Previously committed migrations are immutable; add a new numbered migration: $file" >&2
      failed=1
    fi
    continue
  fi

  if [[ ! -f "$file" ]]; then
    echo "ERROR: New migration is missing from the working tree: $file" >&2
    failed=1
    continue
  fi
  if [[ ! "$file" =~ $migration_file_re ]]; then
    echo "ERROR: New migrations must use apps/<service>[/internal/db]/migrations/NNNN_snake_case.sql: $file" >&2
    failed=1
    continue
  fi
  if ! grep -q -- '-- +goose Up' "$file"; then
    echo "ERROR: Migration is missing a -- +goose Up section: $file" >&2
    failed=1
  fi

  directory="${file%/*}"
  filename="${file##*/}"
  version="${filename%%_*}"
  version_count="$(find "$directory" -maxdepth 1 -type f -name "${version}_*.sql" | wc -l | tr -d ' ')"
  if [[ "$version_count" != "1" ]]; then
    echo "ERROR: Migration version $version is duplicated in $directory" >&2
    failed=1
  fi

  case "$file" in
    apps/identity-service/migrations/*)
      mirror="apps/identity-service/internal/db/migrations/${file##*/}"
      ;;
    apps/identity-service/internal/db/migrations/*)
      mirror="apps/identity-service/migrations/${file##*/}"
      ;;
    *)
      mirror=""
      ;;
  esac
  if [[ -n "$mirror" ]] && { [[ ! -f "$mirror" ]] || ! cmp -s "$file" "$mirror"; }; then
    echo "ERROR: Identity canonical and embedded migrations must be identical: $file <> $mirror" >&2
    failed=1
  fi
done <"$candidate_file"

if [[ "$failed" != "0" ]]; then
  exit 1
fi

echo "Migration history is additive, numbered, forward-defined, and runtime-synchronized."
