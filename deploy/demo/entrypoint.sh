#!/bin/sh
# Supervises the whole AuraEDU backend inside one container.
#
# Each service is an ordinary process listening on its own loopback port, exactly as
# in production — this only collapses 14 containers into one so a demo fits a single
# free instance. Nothing about the service code or its boundaries changes.
#
# Required:  DATABASE_URL  (external, persistent — e.g. Neon free tier)
# Optional:  PORT (Render injects it), INTERNAL_SERVICE_TOKEN, JWT_SIGNING_KEY
set -eu

GATEWAY_PORT="${PORT:-8080}"
: "${DATABASE_URL:?DATABASE_URL is required (point it at a managed PostgreSQL, e.g. Neon)}"

# Ephemeral infrastructure. The demo instance is restarted freely and its caches and
# event log are not durable; only DATABASE_URL holds anything worth keeping.
mkdir -p /tmp/nats
nats-server --jetstream --store_dir /tmp/nats --port 4222 >/tmp/nats.log 2>&1 &
redis-server --port 6379 --save '' --appendonly no >/tmp/redis.log 2>&1 &

# Wait for both before starting anything that fails closed without them.
for _ in $(seq 1 50); do
  if nc -z 127.0.0.1 4222 2>/dev/null && nc -z 127.0.0.1 6379 2>/dev/null; then break; fi
  sleep 0.2
done

export NATS_URL="nats://127.0.0.1:4222"
export REDIS_URL="redis://127.0.0.1:6379"
export ENVIRONMENT="${ENVIRONMENT:-development}"
export INTERNAL_SERVICE_TOKEN="${INTERNAL_SERVICE_TOKEN:-demo-internal-token}"
export JWT_SIGNING_KEY="${JWT_SIGNING_KEY:-demo-signing-key-change-me}"
export FEATURES_REGISTRY=/contracts/features/features.yaml

# Every service shares one database and owns a schema in it (AURA-9.9), so the demo
# needs one managed PostgreSQL rather than one per service.
export DATABASE_MAX_CONNS="${DATABASE_MAX_CONNS:-2}"

pids=""
start() { # start <name> <port>
  name="$1"; port="$2"
  schema="$(echo "$name" | sed 's/-service$//; s/-/_/g')"
  cd "/srv/$name"
  env PORT="$port" DATABASE_SCHEMA="$schema" "/usr/local/bin/$name" server 2>&1 | sed "s/^/[$name] /" &
  pids="$pids $!"
}

# Point every service at its siblings on loopback.
while read -r name port _rest; do
  case "$name" in ''|'#'*) continue ;; esac
  var="SERVICE_$(echo "$name" | sed 's/-service$//; s/-/_/g' | tr '[:lower:]' '[:upper:]')_URL"
  export "$var=http://127.0.0.1:$port"
done < /srv/services.txt

while read -r name port _rest; do
  case "$name" in ''|'#'*) continue ;; esac
  start "$name" "$port"
done < /srv/services.txt

# The gateway is the only process bound to the port Render exposes.
cd /srv
env PORT="$GATEWAY_PORT" /usr/local/bin/api-gateway server 2>&1 | sed 's/^/[gateway] /' &
gateway_pid=$!

# If any process dies the instance is unhealthy, so exit and let the platform restart
# it rather than serve a half-running stack. busybox ash has no `wait -n`, so poll.
trap 'kill $gateway_pid $pids 2>/dev/null; exit 0' TERM INT
while :; do
  for pid in $gateway_pid $pids; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "a backend process ($pid) exited; shutting the demo instance down" >&2
      kill $gateway_pid $pids 2>/dev/null || true
      exit 1
    fi
  done
  sleep 5
done
