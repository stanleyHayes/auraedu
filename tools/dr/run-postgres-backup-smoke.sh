#!/usr/bin/env bash
set -euo pipefail

# End-to-end generated-fixture smoke for the production backup container. It
# never accepts a production URL and removes every temporary resource on exit.

readonly POSTGRES_IMAGE="postgres:18-alpine@sha256:9a8afca54e7861fd90fab5fdf4c42477a6b1cb7d293595148e674e0a3181de15"
readonly BACKUP_IMAGE="auraedu/postgres-backup:verify"
readonly SMOKE_ID="auraedu-pg-backup-$PPID-$$"
readonly NETWORK_NAME="${SMOKE_ID}-network"
readonly DATABASE_CONTAINER="${SMOKE_ID}-db"

smoke_dir="$(mktemp -d "${TMPDIR:-/tmp}/auraedu-postgres-backup.XXXXXX")"
tls_pid=""

cleanup() {
  if [[ -n "$tls_pid" ]]; then
    kill "$tls_pid" >/dev/null 2>&1 || true
  fi
  docker rm -f "$DATABASE_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK_NAME" >/dev/null 2>&1 || true
  rm -rf "$smoke_dir"
}
trap cleanup EXIT INT TERM

docker build -f infrastructure/docker/postgres-backup.Dockerfile -t "$BACKUP_IMAGE" . >/dev/null

openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
  -subj '/CN=host.docker.internal' \
  -addext 'subjectAltName=DNS:host.docker.internal' \
  -keyout "$smoke_dir/tls.key" -out "$smoke_dir/tls.crt" >/dev/null 2>&1

python3 -c '
import http.server, ssl, sys
cert, key, log = sys.argv[1:]
class Handler(http.server.BaseHTTPRequestHandler):
    def receive(self):
        size = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(size)
        with open(log, "a", encoding="utf-8") as handle:
            handle.write(f"{self.command} {self.path} {len(body)}\n")
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"{}")
    do_PUT = receive
    do_POST = receive
    def log_message(self, *_):
        pass
server = http.server.ThreadingHTTPServer(("0.0.0.0", 9443), Handler)
context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
context.load_cert_chain(cert, key)
server.socket = context.wrap_socket(server.socket, server_side=True)
server.serve_forever()
' "$smoke_dir/tls.crt" "$smoke_dir/tls.key" "$smoke_dir/requests.log" &
tls_pid="$!"
sleep 1

docker network create "$NETWORK_NAME" >/dev/null
docker run --detach --name "$DATABASE_CONTAINER" --network "$NETWORK_NAME" \
  --env POSTGRES_PASSWORD=smoke-password --env POSTGRES_DB=identity \
  "$POSTGRES_IMAGE" >/dev/null

for attempt in $(seq 1 60); do
  if docker exec "$DATABASE_CONTAINER" psql -U postgres -d identity -c 'SELECT 1' >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == 60 ]]; then
    docker logs "$DATABASE_CONTAINER" >&2
    exit 1
  fi
  sleep 1
done

docker exec "$DATABASE_CONTAINER" psql -v ON_ERROR_STOP=1 -U postgres -d identity \
  -c "CREATE TABLE recovery_smoke(id integer primary key, payload text not null); INSERT INTO recovery_smoke VALUES (1, 'immutable-export-proof'); CREATE ROLE backup_reader LOGIN PASSWORD 'smoke-password'; GRANT CONNECT ON DATABASE identity TO backup_reader; GRANT USAGE ON SCHEMA public TO backup_reader; GRANT SELECT ON ALL TABLES IN SCHEMA public TO backup_reader;" >/dev/null

docker run --rm --network "$NETWORK_NAME" --add-host host.docker.internal:host-gateway \
  --volume "$smoke_dir/tls.crt:/tmp/recovery-ca.pem:ro" \
  --env SSL_CERT_FILE=/tmp/recovery-ca.pem \
  --env POSTGRES_DATABASES=identity \
  --env "POSTGRES_IDENTITY_DATABASE_URL=postgresql://backup_reader:smoke-password@$DATABASE_CONTAINER:5432/identity?sslmode=disable" \
  --env DR_BACKUP_S3_ENDPOINT=https://host.docker.internal:9443 \
  --env DR_BACKUP_S3_REGION=eu-central-1 \
  --env DR_BACKUP_S3_BUCKET=auraedu-smoke \
  --env DR_BACKUP_S3_PREFIX=auraedu \
  --env DR_BACKUP_S3_ACCESS_KEY_ID=smoke-access \
  --env DR_BACKUP_S3_SECRET_ACCESS_KEY=smoke-secret \
  --env DR_BACKUP_RETENTION_DAYS=35 \
  --env DR_BACKUP_HTTP_TIMEOUT=30s \
  --env DR_POSTGRES_BACKUP_HEARTBEAT_URL=https://host.docker.internal:9443/heartbeat \
  --env DR_POSTGRES_BACKUP_HEARTBEAT_TOKEN=smoke-heartbeat \
  --env DR_POSTGRES_BACKUP_ALERT_URL=https://host.docker.internal:9443/alert \
  --env DR_POSTGRES_BACKUP_ALERT_TOKEN=smoke-alert \
  "$BACKUP_IMAGE"

grep -F 'PUT /auraedu-smoke/auraedu/postgres/' "$smoke_dir/requests.log" | grep -F '/identity-'
grep -F 'POST /heartbeat ' "$smoke_dir/requests.log"
echo 'PostgreSQL backup container smoke passed: live export, catalogue validation, immutable upload and heartbeat.'
