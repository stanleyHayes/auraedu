#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
ENV_FILE="$ROOT_DIR/.env"
COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing $ENV_FILE; run make local-config first" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

"${COMPOSE[@]}" config --services | sort >"$tmp_dir/configured"
"${COMPOSE[@]}" ps -a --services | sort >"$tmp_dir/actual"

if ! diff -u "$tmp_dir/configured" "$tmp_dir/actual"; then
  echo "local runtime does not contain every configured service" >&2
  exit 1
fi

service_count="$(wc -l <"$tmp_dir/configured" | tr -d ' ')"
failures=0
restart_total=0

while IFS= read -r service; do
  container_id="$("${COMPOSE[@]}" ps -aq "$service")"
  state="$(docker inspect "$container_id" --format '{{.State.Status}}')"
  exit_code="$(docker inspect "$container_id" --format '{{.State.ExitCode}}')"
  health="$(docker inspect "$container_id" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}')"
  restarts="$(docker inspect "$container_id" --format '{{.RestartCount}}')"
  restart_total=$((restart_total + restarts))

  if [[ "$service" == "loki-init" ]]; then
    if [[ "$state" != "exited" || "$exit_code" != "0" ]]; then
      echo "FAIL $service: expected successful one-shot exit, got state=$state exit=$exit_code" >&2
      failures=$((failures + 1))
    fi
    continue
  fi

  if [[ "$state" != "running" ]]; then
    echo "FAIL $service: expected running, got $state" >&2
    failures=$((failures + 1))
  elif [[ -n "$health" && "$health" != "healthy" ]]; then
    echo "FAIL $service: health=$health" >&2
    failures=$((failures + 1))
  fi
done <"$tmp_dir/configured"

if (( restart_total > 0 )); then
  echo "FAIL containers restarted $restart_total time(s) since creation" >&2
  failures=$((failures + 1))
fi

probe() {
  local label="$1"
  local url="$2"
  local attempts="${3:-10}"
  local code=""
  local attempt

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$url" || true)"
    if [[ "$code" == "200" ]]; then
      printf 'PASS %-31s %s\n' "$label" "$url"
      return 0
    fi
    sleep 2
  done

  echo "FAIL $label: HTTP ${code:-unreachable} at $url" >&2
  return 1
}

host_address() {
  local service="$1"
  local container_port="$2"
  "${COMPOSE[@]}" port "$service" "$container_port" | tail -n 1 | sed 's/^0\.0\.0\.0/127.0.0.1/; s/^\[::\]/127.0.0.1/'
}

# Probe every published application service, not just the gateway. This catches
# services that are alive as processes but cannot complete readiness checks.
"${COMPOSE[@]}" ps -a --format json | jq -r '
  select((.Service | endswith("-service")) or .Service == "api-gateway")
  | [.Service, ([.Publishers[]? | select(.PublishedPort > 0) | .PublishedPort][0] // 0)]
  | @tsv
' | sort >"$tmp_dir/app-services"

while IFS=$'\t' read -r service port; do
  if [[ "$port" == "0" ]] || ! probe "$service readiness" "http://127.0.0.1:${port}/ready" 3; then
    failures=$((failures + 1))
  fi
done <"$tmp_dir/app-services"

probe "web health" "http://$(host_address web 3000)/api/health" || failures=$((failures + 1))
probe "marketing health" "http://$(host_address marketing 3001)/api/health" || failures=$((failures + 1))
probe "NATS monitoring" "http://$(host_address nats 8222)/healthz" || failures=$((failures + 1))
probe "Prometheus readiness" "http://$(host_address prometheus 9090)/-/ready" || failures=$((failures + 1))
probe "Alertmanager readiness" "http://$(host_address alertmanager 9093)/-/ready" || failures=$((failures + 1))
probe "Grafana health" "http://$(host_address grafana 3000)/api/health" || failures=$((failures + 1))
probe "Loki readiness" "http://$(host_address loki 3100)/ready" || failures=$((failures + 1))
probe "Tempo readiness" "http://$(host_address tempo 3200)/ready" || failures=$((failures + 1))

if (( failures > 0 )); then
  echo "local runtime verification failed with $failures error(s)" >&2
  exit 1
fi

echo "AuraEDU local runtime verified: $service_count services, $restart_total restarts, all application and platform probes passed."
