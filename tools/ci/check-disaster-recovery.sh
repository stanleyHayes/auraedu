#!/usr/bin/env bash
set -euo pipefail

ruby -ryaml <<'RUBY'
blueprint = YAML.load_file('render.yaml')
errors = []
databases = blueprint.fetch('databases', [])

errors << 'render.yaml must declare at least one managed PostgreSQL database' if databases.empty?
databases.each do |database|
  name = database.fetch('name', '<unnamed>')
  plan = database['plan'].to_s
  errors << "#{name}: free/unspecified database plans do not satisfy managed-recovery policy" if plan.empty? || plan == 'free'
  errors << "#{name}: PostgreSQL major version must be explicitly pinned to 18" unless database['postgresMajorVersion'].to_s == '18'
  errors << "#{name}: public database ingress must be denied with ipAllowList: []" unless database['ipAllowList'] == []
end

services = blueprint.fetch('services', [])
nats = services.find { |service| service['name'] == 'nats' }
if nats.nil?
  errors << 'render.yaml must declare the nats private service'
else
  disk = nats['disk'] || {}
  errors << 'nats: JetStream disk must mount at /data' unless disk['mountPath'] == '/data'
  errors << 'nats: JetStream disk must reserve at least 10 GB' unless disk['sizeGB'].to_i >= 10
end

backup = services.find { |service| service['name'] == 'nats-backup' }
if backup.nil?
  errors << 'render.yaml must declare the independent nats-backup cron'
else
  errors << 'nats-backup: must be a cron service' unless backup['type'] == 'cron'
  errors << 'nats-backup: daily schedule must be pinned to 02:15 UTC' unless backup['schedule'] == '15 2 * * *'
  errors << 'nats-backup: must use the dedicated backup Dockerfile' unless backup['dockerfilePath'] == './infrastructure/docker/nats-backup.Dockerfile'
  errors << 'nats-backup: deploys must wait for CI' unless backup['autoDeployTrigger'] == 'checksPass'
  env = backup.fetch('envVars', [])
  group_names = env.map { |entry| entry['fromGroup'] }.compact
  errors << 'nats-backup: recovery secrets must come from auraedu-dr-secrets' unless group_names.include?('auraedu-dr-secrets')
  nats_url = env.find { |entry| entry['key'] == 'NATS_URL' }
  unless nats_url&.dig('fromService', 'name') == 'nats' && nats_url&.dig('fromService', 'property') == 'hostport'
    errors << 'nats-backup: NATS_URL must use the private nats hostport'
  end
end

postgres_backup = services.find { |service| service['name'] == 'postgres-backup' }
if postgres_backup.nil?
  errors << 'render.yaml must declare the independent postgres-backup cron'
else
  errors << 'postgres-backup: must be a cron service' unless postgres_backup['type'] == 'cron'
  errors << 'postgres-backup: hourly schedule must be pinned to minute 17' unless postgres_backup['schedule'] == '17 * * * *'
  errors << 'postgres-backup: must use the dedicated backup Dockerfile' unless postgres_backup['dockerfilePath'] == './infrastructure/docker/postgres-backup.Dockerfile'
  errors << 'postgres-backup: deploys must wait for CI' unless postgres_backup['autoDeployTrigger'] == 'checksPass'
  env = postgres_backup.fetch('envVars', [])
  group_names = env.map { |entry| entry['fromGroup'] }.compact
  errors << 'postgres-backup: recovery secrets must come from auraedu-dr-secrets' unless group_names.include?('auraedu-dr-secrets')
  configured_names = env.find { |entry| entry['key'] == 'POSTGRES_DATABASES' }&.fetch('value', '').to_s.split(',')
  expected_names = databases.map { |database| database.fetch('databaseName') }
  errors << 'postgres-backup: POSTGRES_DATABASES must include every managed database exactly once' unless configured_names == expected_names
  expected_names.each do |database_name|
    key = "POSTGRES_#{database_name.upcase.tr('-', '_')}_DATABASE_URL"
    entry = env.find { |candidate| candidate['key'] == key }
    expected_service = databases.find { |database| database['databaseName'] == database_name }&.fetch('name')
    unless entry&.dig('fromDatabase', 'name') == expected_service && entry&.dig('fromDatabase', 'property') == 'connectionString'
      errors << "postgres-backup: #{key} must use #{expected_service}'s private connection string"
    end
  end
  timeout = env.find { |entry| entry['key'] == 'DR_POSTGRES_BACKUP_JOB_TIMEOUT' }
  errors << 'postgres-backup: job timeout must be pinned to 55m' unless timeout&.fetch('value', nil) == '55m'
end

groups = blueprint.fetch('envVarGroups', []).to_h { |group| [group.fetch('name'), group.fetch('envVars', [])] }
dr_group = groups['auraedu-dr-secrets']
required_dr_keys = %w[
  DR_BACKUP_S3_ENDPOINT DR_BACKUP_S3_REGION DR_BACKUP_S3_BUCKET DR_BACKUP_S3_PREFIX
  DR_BACKUP_S3_ACCESS_KEY_ID DR_BACKUP_S3_SECRET_ACCESS_KEY DR_BACKUP_RETENTION_DAYS
  DR_BACKUP_HEARTBEAT_URL DR_BACKUP_HEARTBEAT_TOKEN DR_BACKUP_ALERT_URL DR_BACKUP_ALERT_TOKEN
  DR_POSTGRES_BACKUP_HEARTBEAT_URL DR_POSTGRES_BACKUP_HEARTBEAT_TOKEN
  DR_POSTGRES_BACKUP_ALERT_URL DR_POSTGRES_BACKUP_ALERT_TOKEN
]
if dr_group.nil?
  errors << 'render.yaml must isolate recovery credentials in auraedu-dr-secrets'
else
  configured = dr_group.map { |entry| entry['key'] }.compact
  missing = required_dr_keys - configured
  errors << "auraedu-dr-secrets: missing #{missing.join(', ')}" unless missing.empty?
end

dockerfile = File.read('infrastructure/docker/nats-backup.Dockerfile')
errors << 'nats-backup image: runtime base must be pinned by digest' unless dockerfile.match?(/^FROM gcr\.io\/distroless\/static-debian12:nonroot@sha256:[0-9a-f]{64}$/)
errors << 'nats-backup image: NATS CLI version must be pinned' unless dockerfile.include?('github.com/nats-io/natscli/nats@v0.3.1')
errors << 'nats-backup image: runtime must be nonroot' unless dockerfile.match?(/^USER nonroot:nonroot$/)

postgres_dockerfile = File.read('infrastructure/docker/postgres-backup.Dockerfile')
postgres_runtime_pattern = Regexp.new('^FROM\\s+postgres:' + '18-alpine@sha256:[0-9a-f]{64}$')
errors << 'postgres-backup image: PostgreSQL 18 runtime must be pinned by digest' unless postgres_dockerfile.match?(postgres_runtime_pattern)
errors << 'postgres-backup image: runtime must use the PostgreSQL nonroot account' unless postgres_dockerfile.match?(/^USER 70:70$/)
errors << 'postgres-backup image: must build the dedicated backup binary' unless postgres_dockerfile.include?('tools/dr/postgres-backup')

abort("Disaster-recovery topology check failed:\n- #{errors.join("\n- ")}") unless errors.empty?
puts "Disaster-recovery topology passed: #{databases.length} paid PostgreSQL 18 databases, hourly logical exports, persistent JetStream and an isolated daily account backup."
RUBY

bash -n tools/dr/run-postgres-restore-drill.sh
bash -n tools/dr/run-postgres-backup-smoke.sh
bash -n tools/dr/run-nats-restore-drill.sh

required_markers=(
  'Tier 0'
  'RPO'
  'RTO'
  'quarterly'
  'Provider evidence still required'
  'run-postgres-restore-drill.sh'
  'run-postgres-backup-smoke.sh'
  'run-nats-restore-drill.sh'
  'nats-backup'
  'postgres-backup'
  'success heartbeat'
  '75-minute'
)

for marker in "${required_markers[@]}"; do
  if ! grep -Fq "$marker" docs/engineering-handbook/04-operations/32-disaster-recovery.md; then
    echo "ERROR: disaster-recovery chapter is missing required marker: $marker" >&2
    exit 1
  fi
done

test -s docs/engineering-handbook/04-operations/runbooks/disaster-recovery.md
echo "Disaster-recovery policy and executable drill checks passed."
