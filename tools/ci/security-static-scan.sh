#!/usr/bin/env bash
set -euo pipefail

# Fast, deterministic preflight for tracked source. This complements dependency
# advisory tools and host-side secret scanning without sending repository data to
# a third party. Intentional public fixtures require an inline `security-scan:
# allow <reason>` annotation so exceptions remain reviewable.

failed=0

secret_pattern='-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|sk_(live|test)_[A-Za-z0-9]{16,}|AIza[0-9A-Za-z_-]{30,}|AC[0-9a-fA-F]{32}'
secret_matches="$(git grep -nI -E -- "$secret_pattern" -- ':!pnpm-lock.yaml' ':!uv.lock' ':!**/go.sum' 2>/dev/null || true)"
secret_matches="$(printf '%s\n' "$secret_matches" | grep -v 'security-scan: allow' || true)"
if [[ -n "${secret_matches//$'\n'/}" ]]; then
  echo "ERROR: credential-like material found in tracked files:" >&2
  printf '%s\n' "$secret_matches" >&2
  failed=1
fi

# Raw event/request bodies and direct sensitive fields must never be application
# log arguments. IDs, counts, status codes and byte lengths are safe metadata.
log_pattern='(logger|logging|console)\.(debug|info|warning|warn|error|exception)\([^\n]*(payload\.decode\(|json\.dumps\((payload|body|request)|JSON\.stringify\((payload|body|request)|%[srov][^\n]*,[[:space:]]*(payload|body|request|email|phone|password|access_token|refresh_token|secret)([^A-Za-z0-9_]|$))|slog\.[A-Za-z]+Context\([^\n]*"(email|phone|password|access_token|refresh_token|secret|payload)"[[:space:]]*,'
log_matches="$(git grep -nI -E -- "$log_pattern" -- 'apps/**/*.go' 'apps/**/*.py' 'apps/**/*.ts' 'apps/**/*.tsx' 'platform/**/*.go' 'packages/**/*.ts' 'packages/**/*.tsx' 2>/dev/null || true)"
log_matches="$(printf '%s\n' "$log_matches" | grep -v 'security-scan: allow' || true)"
if [[ -n "${log_matches//$'\n'/}" ]]; then
  echo "ERROR: potentially sensitive application log arguments found:" >&2
  printf '%s\n' "$log_matches" >&2
  failed=1
fi

raw_query_logs="$(git grep -nI -E -- '"query"[[:space:]]*,[[:space:]]*r\.URL\.RawQuery|slog\.[A-Za-z]+Context\([^\n]*RawQuery' -- 'apps/**/*.go' 'platform/**/*.go' 2>/dev/null || true)"
if [[ -n "$raw_query_logs" ]]; then
  echo "ERROR: raw query strings must not be written to application logs:" >&2
  printf '%s\n' "$raw_query_logs" >&2
  failed=1
fi

# Production Go processes must use platform/observ's runtime redactor. Raw slog
# JSON handlers are allowed in tests where buffers/stdout are explicit fixtures.
raw_go_logger_pattern='slog\.New\(slog\.New(JSON|Text)Handler\('
raw_go_loggers="$(git grep -nI -E -- "$raw_go_logger_pattern" -- 'apps/**/cmd/**/*.go' ':!apps/**/cmd/**/*_test.go' 2>/dev/null || true)"
if [[ -n "${raw_go_loggers//$'\n'/}" ]]; then
  echo "ERROR: production Go entrypoints bypass the PII-redacting logger:" >&2
  printf '%s\n' "$raw_go_loggers" >&2
  failed=1
fi

tracked_env="$(git ls-files | grep -E '(^|/)(\.env|[^/]+\.env)$' | grep -vE '(^|/)(\.env\.example|example\.env)$' || true)"
if [[ -n "$tracked_env" ]]; then
  echo "ERROR: runtime environment files are tracked:" >&2
  printf '%s\n' "$tracked_env" >&2
  failed=1
fi

# A mutable action tag or branch lets upstream code change without a reviewed
# repository diff. Local reusable workflows are trusted from this checkout;
# every remote action/workflow must resolve to an immutable 40-character SHA.
workflow_action_failures=""
while IFS= read -r workflow_use; do
  if [[ "$workflow_use" == *"uses: ./"* ]]; then
    continue
  fi
  if ! grep -Eq 'uses:[[:space:]]+[^[:space:]#]+@[0-9a-f]{40}([[:space:]]+#.*)?$' <<<"$workflow_use"; then
    workflow_action_failures="${workflow_action_failures}${workflow_use}\n"
  fi
done < <(grep -Hn -E '^[[:space:]-]*uses:[[:space:]]+' .github/workflows/*.yml || true)
if [[ -n "$workflow_action_failures" ]]; then
  echo "ERROR: remote GitHub Actions must be pinned to immutable commit SHAs:" >&2
  printf '%b' "$workflow_action_failures" >&2
  failed=1
fi

manifest_image_failures="$(grep -Hn -E '^[[:space:]]+image:[[:space:]]+' .github/workflows/*.yml deploy/*.yml | grep -vE '@sha256:[0-9a-f]{64}([[:space:]]|$)' || true)"
if [[ -n "$manifest_image_failures" ]]; then
  echo "ERROR: CI and deployment service/container images must be pinned by digest:" >&2
  printf '%s\n' "$manifest_image_failures" >&2
  failed=1
fi

script_image_failures="$(grep -Hn -E '(postgres|valkey/valkey|nats(io/nats-box)?|prom/prometheus|prom/alertmanager|otel/opentelemetry-collector-contrib|grafana/(loki|tempo|alloy|grafana)):[A-Za-z0-9._-]+' tools/ci/*.sh tools/dr/*.sh tools/smoke/*.sh | grep -vE '@sha256:[0-9a-f]{64}([^0-9a-f]|$)' || true)"
if [[ -n "$script_image_failures" ]]; then
  echo "ERROR: CI script container images must be pinned by digest:" >&2
  printf '%s\n' "$script_image_failures" >&2
  failed=1
fi

testcontainer_image_failures="$(git grep -nI -E -- 'postgres\.Run\([^\n]*"postgres:[^"]+"' -- 'apps/**/*.go' 'platform/**/*.go' 'tools/**/*.go' | grep -vE '@sha256:[0-9a-f]{64}' || true)"
if [[ -n "$testcontainer_image_failures" ]]; then
  echo "ERROR: Go testcontainer images must be pinned by digest:" >&2
  printf '%s\n' "$testcontainer_image_failures" >&2
  failed=1
fi

mutable_ci_tools="$(grep -Hn -E '@latest([[:space:]]|$)|go-version:[^#]*stable|npm install -g (@[^/[:space:]]+/)?[^@[:space:]]+([[:space:]]|$)' .github/workflows/*.yml || true)"
if [[ -n "$mutable_ci_tools" ]]; then
  echo "ERROR: CI tools must use explicit versions:" >&2
  printf '%s\n' "$mutable_ci_tools" >&2
  failed=1
fi

runnable_jobs="$(grep -h -E '^[[:space:]]+runs-on:[[:space:]]+' .github/workflows/*.yml | wc -l | tr -d ' ')"
bounded_jobs="$(grep -h -E '^[[:space:]]+timeout-minutes:[[:space:]]+[1-9][0-9]*[[:space:]]*$' .github/workflows/*.yml | wc -l | tr -d ' ')"
if [[ "$runnable_jobs" != "$bounded_jobs" ]]; then
  echo "ERROR: every runnable CI job must have one positive timeout-minutes value (jobs=$runnable_jobs timeouts=$bounded_jobs)." >&2
  failed=1
fi

insecure_auth_defaults="$(git grep -nI -E -- 'JWT_SIGNING_KEY[^\n]*(dev|default|fallback|change-me)|dev-insecure-signing-key' -- 'apps/**/*.go' 2>/dev/null || true)"
if [[ -n "$insecure_auth_defaults" ]]; then
  echo "ERROR: production JWT signing-key fallback found; auth processes must fail startup when the key is absent:" >&2
  printf '%s\n' "$insecure_auth_defaults" >&2
  failed=1
fi

# Every production Go HTTP server must compose the shared ingress middleware.
# It generates correlation IDs and enforces the global 1 MiB request-body cap
# (40 MiB only for multipart uploads) before service-specific stricter limits.
unbounded_go_servers=""
while IFS= read -r server_file; do
  if grep -q 'http\.Server[[:space:]]*{' "$server_file" && ! grep -Eq 'httpx\.(RequestID|RequestBoundary)Middleware' "$server_file"; then
    unbounded_go_servers="${unbounded_go_servers}${server_file}: missing shared request-boundary middleware\n"
  fi
done < <(find apps -path '*/cmd/server/server.go' -type f | sort)
if [[ -n "$unbounded_go_servers" ]]; then
  echo "ERROR: production Go HTTP servers can accept unbounded request bodies:" >&2
  printf '%b' "$unbounded_go_servers" >&2
  failed=1
fi

unbounded_internal_responses="$(git grep -nI -E -- 'json\.NewDecoder\([^)]*\.Body\)|io\.ReadAll\([^)]*\.Body\)' -- 'apps/**/internal/adapters/**/client.go' 2>/dev/null || true)"
if [[ -n "$unbounded_internal_responses" ]]; then
  echo "ERROR: internal service clients must use a bounded response decoder:" >&2
  printf '%s\n' "$unbounded_internal_responses" >&2
  failed=1
fi

unbounded_python_responses="$(git grep -nI -E -- 'json\.load\([^)]*(response|resp)|\.(read|readall)\(\)' -- \
  'apps/ai-recommendation-service/src/**/*.py' \
  'apps/ai-prediction-service/src/**/*.py' \
  'apps/career-guidance-service/src/**/*.py' 2>/dev/null || true)"
if [[ -n "$unbounded_python_responses" ]]; then
  echo "ERROR: Python AI dependency responses must use an explicit byte ceiling:" >&2
  printf '%s\n' "$unbounded_python_responses" >&2
  failed=1
fi

unbounded_python_ingress=""
for python_app in \
  apps/ai-recommendation-service/src/ai_recommendation_service/main.py \
  apps/ai-prediction-service/src/ai_prediction_service/main.py \
  apps/career-guidance-service/src/career_guidance_service/main.py; do
  if ! grep -q 'add_middleware(RequestBodyLimitMiddleware)' "$python_app"; then
    unbounded_python_ingress="${unbounded_python_ingress}${python_app}: missing request-body limit middleware\n"
  fi
done
if [[ -n "$unbounded_python_ingress" ]]; then
  echo "ERROR: Python AI HTTP services can accept unbounded request bodies:" >&2
  printf '%b' "$unbounded_python_ingress" >&2
  failed=1
fi

# Production consumers must use the shared Go eventbus policy (manual ack,
# bounded redelivery, poison termination and DLQ) instead of raw subscriptions.
direct_go_subscribers="$(find apps -type f -name '*.go' ! -name '*_test.go' -print0 | xargs -0 grep -Hn -E '\.js\.Subscribe\(|js\.Subscribe\(' 2>/dev/null || true)"
if [[ -n "$direct_go_subscribers" ]]; then
  echo "ERROR: production Go consumers bypass the shared retry/DLQ policy:" >&2
  printf '%s\n' "$direct_go_subscribers" >&2
  failed=1
fi

if ! grep -q 'len(data) > MaxEventBytes' platform/eventbus/eventbus.go || ! grep -q 'ErrEventTooLarge' platform/eventbus/eventbus.go; then
  echo "ERROR: the shared Go event publisher does not reject oversized envelopes before NATS." >&2
  failed=1
fi

unbounded_ai_consumers=""
for subscriber_file in \
  apps/ai-recommendation-service/src/ai_recommendation_service/events/subscriber.py \
  apps/ai-prediction-service/src/ai_prediction_service/events/subscriber.py \
  apps/career-guidance-service/src/career_guidance_service/events/subscriber.py; do
  if ! grep -q 'manual_ack=True' "$subscriber_file" || ! grep -q 'max_deliver=MAX_DELIVERIES' "$subscriber_file" || ! grep -q '_reconcile_consumer' "$subscriber_file" || ! grep -q '_ensure_stream' "$subscriber_file" || ! grep -q 'max_msgs=STREAM_MAX_MESSAGES' "$subscriber_file" || ! grep -q 'max_age=STREAM_MAX_AGE_SECONDS' "$subscriber_file"; then
    unbounded_ai_consumers="${unbounded_ai_consumers}${subscriber_file}: missing manual ack, bounded delivery, durable reconciliation or bounded stream reconciliation\n"
  fi
  if grep -Eq '"(attendance\.marked|analytics\.metric_updated)"' "$subscriber_file"; then
    unbounded_ai_consumers="${unbounded_ai_consumers}${subscriber_file}: subscribes to an unversioned event type\n"
  fi
done
if [[ -n "$unbounded_ai_consumers" ]]; then
  echo "ERROR: Python AI consumers bypass the bounded retry policy:" >&2
  printf '%b' "$unbounded_ai_consumers" >&2
  failed=1
fi

unbounded_ai_publishers=""
for publisher_file in \
  apps/ai-recommendation-service/src/ai_recommendation_service/events/publisher.py \
  apps/ai-prediction-service/src/ai_prediction_service/events/publisher.py \
  apps/ai-prediction-service/src/ai_prediction_service/events/outbox.py \
  apps/career-guidance-service/src/career_guidance_service/events/publisher.py \
  apps/career-guidance-service/src/career_guidance_service/events/outbox.py; do
  if ! grep -q 'encode_event(' "$publisher_file"; then
    unbounded_ai_publishers="${unbounded_ai_publishers}${publisher_file}: missing bounded event encoding\n"
  fi
done
for envelope_file in \
  apps/ai-recommendation-service/src/ai_recommendation_service/events/envelope.py \
  apps/ai-prediction-service/src/ai_prediction_service/events/envelope.py \
  apps/career-guidance-service/src/career_guidance_service/events/envelope.py; do
  if ! grep -q 'MAX_EVENT_BYTES = 1 << 20' "$envelope_file" || ! grep -q 'len(payload) > MAX_EVENT_BYTES' "$envelope_file"; then
    unbounded_ai_publishers="${unbounded_ai_publishers}${envelope_file}: missing the 1 MiB outbound envelope ceiling\n"
  fi
done
if [[ -n "$unbounded_ai_publishers" ]]; then
  echo "ERROR: Python AI publishers can emit unbounded event envelopes:" >&2
  printf '%b' "$unbounded_ai_publishers" >&2
  failed=1
fi

docker_user_failures=""
docker_digest_failures=""
while IFS= read -r dockerfile; do
  final_from_line="$(grep -n '^FROM ' "$dockerfile" | tail -1 | cut -d: -f1)"
  final_user="$(tail -n "+$final_from_line" "$dockerfile" | grep '^USER ' | tail -1 | awk '{print $2}' || true)"
  case "$final_user" in
    ""|root|0|0:0) docker_user_failures="${docker_user_failures}${dockerfile}: final runtime stage has no non-root USER\n" ;;
  esac
  unpinned="$(grep '^FROM ' "$dockerfile" | grep -vE '@sha256:[0-9a-f]{64}([[:space:]]|$)' || true)"
  if [[ -n "$unpinned" ]]; then
    docker_digest_failures="${docker_digest_failures}${dockerfile}: mutable base image reference ${unpinned}\n"
  fi
done < <(find apps infrastructure -type f \( -name Dockerfile -o -name '*.Dockerfile' \) | sort)
if [[ -n "$docker_user_failures" ]]; then
  echo "ERROR: root-capable production container runtimes found:" >&2
  printf '%b' "$docker_user_failures" >&2
  failed=1
fi
if [[ -n "$docker_digest_failures" ]]; then
  echo "ERROR: production container base images must be pinned by digest:" >&2
  printf '%b' "$docker_digest_failures" >&2
  failed=1
fi

if [[ "$failed" != "0" ]]; then
  exit 1
fi

echo "Static security scan passed: no credential signatures, tracked runtime env files, insecure JWT fallbacks, raw sensitive log arguments, unredacted Go entrypoints, unbounded Go/Python request bodies, internal responses or event envelopes, mutable CI actions/tools/images, unbounded CI jobs, root container runtimes, or mutable base image references."
