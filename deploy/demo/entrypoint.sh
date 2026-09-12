#!/bin/sh
# AURA-9.12: supervise actual children, isolate service databases and listeners.
set -eu
GATEWAY_PORT="${PORT:-8080}"
DATABASE_DRIVER="$(printf '%s' "${DATABASE_DRIVER:-postgres}" | tr '[:upper:]' '[:lower:]' | xargs)"
export DATABASE_DRIVER
case "$DATABASE_DRIVER" in
 mongodb|mongo) : "${MONGODB_URI:?MONGODB_URI required}"; export DATABASE_DRIVER=mongodb ;;
 postgres|postgresql|pg|'') : "${DATABASE_URL:?DATABASE_URL required}" ;;
 *) echo 'unsupported DATABASE_DRIVER' >&2; exit 1 ;;
esac
export NATS_URL=nats://127.0.0.1:4222 REDIS_URL=redis://127.0.0.1:6379
export ENVIRONMENT="${ENVIRONMENT:-development}"
: "${INTERNAL_SERVICE_TOKEN:?explicit INTERNAL_SERVICE_TOKEN required}"
: "${JWT_SIGNING_KEY:?explicit JWT_SIGNING_KEY required}"
export INTERNAL_SERVICE_TOKEN JWT_SIGNING_KEY
export FEATURES_REGISTRY=/contracts/features/features.yaml
export DATABASE_MAX_CONNS="${DATABASE_MAX_CONNS:-2}"
export MONGODB_DATABASE_PREFIX="${MONGODB_DATABASE_PREFIX:-auraedu_demo}"
pids=""
shutdown() {
 trap - TERM INT EXIT
 for pid in $pids; do kill "$pid" 2>/dev/null || true; done
 elapsed=0
 while [ "$elapsed" -lt 15 ]; do
  remaining=""; for pid in $pids; do if kill -0 "$pid" 2>/dev/null; then remaining="$remaining $pid"; fi; done
  [ -n "$remaining" ] || break
  sleep 1; elapsed=$((elapsed+1))
 done
 for pid in $pids; do kill -KILL "$pid" 2>/dev/null || true; done
 for pid in $pids; do wait "$pid" 2>/dev/null || true; done
}
trap 'exit 0' TERM INT
trap shutdown EXIT
alive() {
 for pid in $pids; do
  if ! kill -0 "$pid" 2>/dev/null; then echo "demo child $pid exited" >&2; exit 1; fi
 done
}
wait_http() {
 url="$1"; attempts=0
 until wget -T 2 -q -O /dev/null "$url"; do
  alive; attempts=$((attempts+1))
  if [ "$attempts" -ge 120 ]; then echo "readiness timed out: $url" >&2; exit 1; fi
  sleep 1
 done
}
mkdir -p /tmp/nats /tmp/reports
nats-server --addr 127.0.0.1 --jetstream --store_dir /tmp/nats --port 4222 &
pids="$pids $!"
redis-server --bind 127.0.0.1 --port 6379 --save '' --appendonly no &
pids="$pids $!"
attempts=0
until nc -z 127.0.0.1 4222 && nc -z 127.0.0.1 6379; do
 alive; attempts=$((attempts+1)); [ "$attempts" -lt 60 ] || exit 1; sleep 1
done
while read -r name port rest; do
 case "$name" in ''|'#'*) continue ;; esac
 key="$(echo "$name" | sed 's/-service$//;s/-/_/g' | tr '[:lower:]' '[:upper:]')"
 export "SERVICE_${key}_URL=http://127.0.0.1:$port"
done < /srv/services.txt
start() {
 name="$1"; port="$2"; command="$3"
 schema="$(echo "$name" | sed 's/-service$//;s/-/_/g')"
 (cd "/srv/$name"; exec env BIND_HOST=127.0.0.1 PORT="$port" \
   DATABASE_SCHEMA="$schema" MONGODB_DATABASE="${MONGODB_DATABASE_PREFIX}_${schema}" \
   REPORT_OUTPUT_DIR=/tmp/reports "/usr/local/bin/$name" "$command") &
 pids="$pids $!"
}
# All servers must pass readiness before workers or public gateway start.
while read -r name port rest; do
 case "$name" in ''|'#'*) continue ;; esac
 start "$name" "$port" server
done < /srv/services.txt
while read -r name port rest; do
 case "$name" in ''|'#'*) continue ;; esac
 wait_http "http://127.0.0.1:$port/ready"
done < /srv/services.txt
while read -r name port rest; do
 case "$name" in ''|'#'*) continue ;; esac
 start "$name" "$((port+1000))" worker
done < /srv/services.txt
(cd /srv; exec env BIND_HOST=0.0.0.0 PORT="$GATEWAY_PORT" /usr/local/bin/api-gateway server) &
pids="$pids $!"
wait_http "http://127.0.0.1:$GATEWAY_PORT/ready"
echo 'demo servers and gateway ready; workers supervised'
while :; do alive; sleep 2; done
