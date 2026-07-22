#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="$repo_root/deploy/docker-compose.infra.yml"

export GRAFANA_PORT="${GRAFANA_PORT:-3300}"
export PROMETHEUS_PORT="${PROMETHEUS_PORT:-39090}"
grafana_user="${GRAFANA_ADMIN_USER:-admin}"
grafana_password="${GRAFANA_ADMIN_PASSWORD:-auraedu-local-change-me}"

wait_for_url() {
  local name="$1"
  local url="$2"
  local attempts="${3:-30}"
  local delay="${4:-2}"

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl --fail --silent --show-error "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$delay"
  done

  echo "$name did not become ready at $url" >&2
  return 1
}

if [[ "${AURA_OBSERVABILITY_SMOKE_SKIP_UP:-0}" != "1" ]]; then
  docker compose -f "$compose_file" up -d --wait \
    alertmanager loki tempo prometheus alloy grafana otel-collector
fi

wait_for_url Prometheus "http://127.0.0.1:${PROMETHEUS_PORT}/-/ready"
wait_for_url Alertmanager "http://127.0.0.1:9093/-/ready"
wait_for_url Loki "http://127.0.0.1:3100/ready"
wait_for_url Tempo "http://127.0.0.1:3200/ready"
wait_for_url Alloy "http://127.0.0.1:12345/-/ready"
wait_for_url Grafana "http://127.0.0.1:${GRAFANA_PORT}/api/health"
wait_for_url OTel-Collector "http://127.0.0.1:13133/"

alloy_targets=""
for ((attempt = 1; attempt <= 30; attempt++)); do
  alloy_targets="$(curl --fail --silent --show-error \
    "http://127.0.0.1:12345/api/v0/web/components/discovery.relabel.auraedu_logs")"
  if ALLOY_TARGETS="$alloy_targets" python3 - <<'PY' >/dev/null 2>&1
import json
import os

payload = json.loads(os.environ["ALLOY_TARGETS"])
output = next(item["value"]["value"] for item in payload["exports"] if item["name"] == "output")
if not output:
    raise SystemExit(1)
PY
  then
    break
  fi
  sleep 2
done

ALLOY_TARGETS="$alloy_targets" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["ALLOY_TARGETS"])
output = next(item["value"]["value"] for item in payload["exports"] if item["name"] == "output")
projects = set()
for target in output:
    labels = {item["key"]: item["value"]["value"] for item in target["value"]}
    projects.add(labels.get("compose_project"))
if not output:
    raise SystemExit("Alloy did not discover any AuraEDU log targets")
if not projects <= {"auraedu", "auraedu-infra"}:
    raise SystemExit(f"Alloy leaked non-AuraEDU projects: {sorted(projects)}")
print(f"Alloy: {len(output)} filtered targets across projects={sorted(projects)}")
PY

prometheus_rules="$(curl --fail --silent --show-error \
  "http://127.0.0.1:${PROMETHEUS_PORT}/api/v1/rules?type=alert")"
PROMETHEUS_RULES="$prometheus_rules" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["PROMETHEUS_RULES"])
rules = [rule for group in payload["data"]["groups"] for rule in group["rules"]]
if len(rules) != 9:
    raise SystemExit(f"expected 9 active Prometheus alert rules, found {len(rules)}")
unhealthy = [rule["name"] for rule in rules if rule.get("health") != "ok"]
if unhealthy:
    raise SystemExit(f"unhealthy Prometheus alert rules: {', '.join(unhealthy)}")
print("Prometheus: 9 alert rules loaded and healthy")
PY

grafana_datasources="$(curl --fail --silent --show-error \
  --user "${grafana_user}:${grafana_password}" \
  "http://127.0.0.1:${GRAFANA_PORT}/api/datasources")"
GRAFANA_DATASOURCES="$grafana_datasources" python3 - <<'PY'
import json
import os

datasources = json.loads(os.environ["GRAFANA_DATASOURCES"])
actual = {(item["uid"], item["type"]) for item in datasources}
expected = {("prometheus", "prometheus"), ("loki", "loki"), ("tempo", "tempo")}
missing = expected - actual
if missing:
    raise SystemExit(f"missing Grafana datasources: {sorted(missing)}")
print("Grafana: Prometheus, Loki, and Tempo datasources provisioned")
PY

grafana_dashboards="$(curl --fail --silent --show-error \
  --user "${grafana_user}:${grafana_password}" \
  "http://127.0.0.1:${GRAFANA_PORT}/api/search?query=AuraEDU%20Golden%20Signals")"
GRAFANA_DASHBOARDS="$grafana_dashboards" python3 - <<'PY'
import json
import os

dashboards = json.loads(os.environ["GRAFANA_DASHBOARDS"])
if not any(item.get("uid") == "auraedu-golden-signals" for item in dashboards):
    raise SystemExit("AuraEDU Golden Signals dashboard was not provisioned")
print("Grafana: AuraEDU Golden Signals dashboard provisioned")
PY

echo "AuraEDU observability stack smoke test passed"
