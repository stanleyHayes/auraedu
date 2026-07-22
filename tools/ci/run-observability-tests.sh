#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

go_bin=${GO_BIN:-}
if [[ -z "$go_bin" ]]; then
  go_bin=$(command -v go || true)
fi
if [[ -z "$go_bin" && -x /opt/homebrew/bin/go ]]; then
  go_bin=/opt/homebrew/bin/go
fi
if [[ -z "$go_bin" ]]; then
  echo "go executable not found" >&2
  exit 1
fi
export GOCACHE=${GOCACHE:-${TMPDIR:-/tmp}/auraedu-observability-go-cache}
mkdir -p "$GOCACHE"

echo "::group::shared observability behavior"
(cd platform && GOWORK=off "$go_bin" test ./observ -count=1)
echo "::endgroup::"

echo "::group::server instrumentation inventory"
servers=(apps/*-service/cmd/server/server.go apps/api-gateway/cmd/server/server.go)
if [[ ${#servers[@]} -ne 25 ]]; then
  echo "expected 25 Go HTTP server entrypoints, found ${#servers[@]}" >&2
  exit 1
fi
for server in "${servers[@]}"; do
  grep -q 'observ.HTTPHandler' "$server" || { echo "$server lacks shared HTTP metrics" >&2; exit 1; }
  grep -q 'observ.InitTracing' "$server" || { echo "$server lacks trace initialization" >&2; exit 1; }
done
echo "25 Go HTTP services expose canonical metrics and initialize graceful OTLP tracing"
python_metrics=(
  apps/ai-recommendation-service/src/ai_recommendation_service/observability.py
  apps/ai-prediction-service/src/ai_prediction_service/observability.py
  apps/career-guidance-service/src/career_guidance_service/observability.py
)
python_apps=(
  apps/ai-recommendation-service/src/ai_recommendation_service/main.py
  apps/ai-prediction-service/src/ai_prediction_service/main.py
  apps/career-guidance-service/src/career_guidance_service/main.py
)
for module in "${python_metrics[@]}"; do
  grep -q 'class PrometheusMiddleware' "$module" || { echo "$module lacks Prometheus middleware" >&2; exit 1; }
done
for app in "${python_apps[@]}"; do
  grep -Eq 'add_middleware\(PrometheusMiddleware, service="[a-z0-9-]+"\)' "$app" || {
    echo "$app does not install Prometheus middleware" >&2
    exit 1
  }
done
echo "3 Python AI HTTP services expose the same canonical golden signals"
grep -q 'auraedu.notification.deliveries' apps/notification-service/internal/application/metrics.go || {
  echo "notification provider delivery metric is missing" >&2
  exit 1
}
active_workers=(
	apps/ai-orchestrator-service/cmd/worker/worker.go
	apps/academic-service/cmd/worker/worker.go
	apps/assessment-service/cmd/worker/worker.go
  apps/admissions-service/cmd/worker/worker.go
  apps/analytics-service/cmd/worker/worker.go
  apps/audit-service/cmd/worker/worker.go
	apps/attendance-service/cmd/worker/worker.go
  apps/billing-service/cmd/worker/worker.go
  apps/campaign-service/cmd/worker/worker.go
	apps/content-service/cmd/worker/worker.go
	apps/cbt-service/cmd/worker/worker.go
  apps/crm-service/cmd/worker/worker.go
  apps/fees-service/cmd/worker/worker.go
	apps/file-service/cmd/worker/worker.go
  apps/identity-service/cmd/worker/worker.go
  apps/knowledge-service/cmd/worker/worker.go
  apps/market-intelligence-service/cmd/worker/worker.go
  apps/notification-service/cmd/worker/worker.go
	apps/payment-service/cmd/worker/worker.go
	apps/report-service/cmd/worker/worker.go
	apps/staff-service/cmd/worker/worker.go
	apps/student-service/cmd/worker/worker.go
	apps/tenant-service/cmd/worker/worker.go
  apps/website-service/cmd/worker/worker.go
)
for worker in "${active_workers[@]}"; do
  grep -q 'InitTracing(' "$worker" || { echo "$worker does not initialize OTLP telemetry" >&2; exit 1; }
  grep -q 'NewWorkerMetrics(' "$worker" || { echo "$worker does not record bounded job outcomes" >&2; exit 1; }
  worker_module=${worker%%/cmd/worker/worker.go}
  (cd "$worker_module" && GOWORK=off GOFLAGS=-mod=readonly "$go_bin" test ./cmd/worker -run '^$' -count=1)
done
echo "${#active_workers[@]} active Go workers export bounded job outcomes through OTLP telemetry"
echo "notification provider outcomes export through the same telemetry pipeline"
echo "::endgroup::"

echo "::group::configuration syntax"
docker compose -f deploy/docker-compose.infra.yml config >/dev/null
docker compose -f deploy/docker-compose.yml config >/dev/null
ruby -e 'require "yaml"; ARGV.each { |f| YAML.load_file(f) }' \
  infrastructure/observability/prometheus.yml \
  infrastructure/observability/alertmanager.yml \
  infrastructure/observability/rules/auraedu-alerts.yml \
  infrastructure/observability/loki.yml \
  infrastructure/observability/tempo.yml \
  infrastructure/docker/otel-collector-config.yaml \
  infrastructure/observability/grafana/provisioning/datasources/auraedu.yml \
  infrastructure/observability/grafana/provisioning/dashboards/auraedu.yml
python3 -m json.tool infrastructure/observability/grafana/dashboards/golden-signals.json >/dev/null
ruby -e '
  require "yaml"
  groups = YAML.load_file(ARGV.fetch(0)).fetch("groups")
  rules = groups.flat_map { |group| group.fetch("rules") }
  abort "observability alerts are missing" if rules.empty?
  rules.each do |rule|
    name = rule.fetch("alert")
    labels = rule.fetch("labels")
    annotations = rule.fetch("annotations")
    abort "#{name} lacks severity/team ownership" unless labels["severity"] && labels["team"]
    abort "#{name} lacks a runbook" unless annotations["runbook"]&.start_with?("docs/")
  end
' infrastructure/observability/rules/auraedu-alerts.yml
echo "::endgroup::"

if [[ ${AURA_OBSERVABILITY_CONTAINER_VALIDATE:-0} == 1 ]]; then
  echo "::group::native configuration validators"
  docker run --rm \
    -v "$repo_root/infrastructure/observability:/etc/prometheus:ro" \
    --entrypoint /bin/promtool prom/prometheus:v3.12.0@sha256:69f5241418838263316593f7274a304b095c40bcf22e57272865da91bd60a8ac \
    check config /etc/prometheus/prometheus.yml
  docker run --rm \
    -v "$repo_root/infrastructure/observability/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro" \
    --entrypoint /bin/amtool prom/alertmanager:v0.32.1@sha256:51a825c2a40acc3e338fdd00d622e01ec090f72be2b3ea46be0839cd47a4d286 \
    check-config /etc/alertmanager/alertmanager.yml
  docker run --rm \
    -v "$repo_root/infrastructure/docker/otel-collector-config.yaml:/etc/otel/config.yaml:ro" \
    --entrypoint /otelcol-contrib otel/opentelemetry-collector-contrib:0.156.0@sha256:125bdbeb7590cc1952c5b3430ecf14063568980c2c93d5b38676cc0446ed8108 \
    validate --config=/etc/otel/config.yaml
  docker run --rm \
    -v "$repo_root/infrastructure/observability/loki.yml:/etc/loki/config.yml:ro" \
    grafana/loki:3.7.2@sha256:191d4fdfb7264f16989f0a57f320872620a5a7c2ceeec6229212c4190ec49b86 -verify-config -config.file=/etc/loki/config.yml
  docker run --rm \
    -v "$repo_root/infrastructure/observability/tempo.yml:/etc/tempo/config.yml:ro" \
    grafana/tempo:2.10.5@sha256:ee21727732c7a7199cb71c3eee9153bbf23f9b0b87619f0555a0cf21a67f1a33 -config.file=/etc/tempo/config.yml -config.verify=true
  docker run --rm \
    -v "$repo_root/infrastructure/observability/alloy.config:/etc/alloy/config.alloy:ro" \
    --entrypoint /bin/alloy grafana/alloy:v1.16.1@sha256:51aeb9d829239345070619dad3edd6873186f913c84f45b365b74574fcb38ec0 \
    fmt --test /etc/alloy/config.alloy
  echo "::endgroup::"
fi

echo "Observability gate passed."
