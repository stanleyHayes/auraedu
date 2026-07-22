#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

ruby -ryaml <<'RUBY'
require "json"

render = YAML.load_file("render.yaml")
groups = render.fetch("envVarGroups", []).to_h do |group|
  keys = group.fetch("envVars", []).map { |entry| entry["key"] }.compact
  [group.fetch("name"), keys]
end
services = render.fetch("services").to_h { |service| [service.fetch("name"), service] }
compose = YAML.load_file("deploy/docker-compose.yml")
compose_services = compose.fetch("services")

required = Hash.new { |deployments, name| deployments[name] = Hash.new { |vars, key| vars[key] = [] } }
sources = Dir.glob("apps/*-service/**/*.go") + Dir.glob("apps/api-gateway/**/*.go")
sources.sort.each do |path|
  next if path.end_with?("_test.go") || path.include?("/tests/")

  variables = File.read(path).scan(/MustGetenv\("([A-Z0-9_]+)"\)/).flatten
  next if variables.empty?

  app = path.split("/")[1]
  deployment = path.include?("/cmd/worker/") ? app.sub(/-service\z/, "-worker") : app
  variables.each { |variable| required[deployment][variable] << path }
end

failures = []

Dir.glob("apps/*-service/internal/adapters/**/*.go").sort.each do |path|
  next if path.end_with?("_test.go")
  source = File.read(path)
  next unless source.match?(/func New\w+\(baseURL(?:,\s*\w+)? string/)
  next unless source.include?("http.NewRequest")
  unless source.include?("config.ServiceURL(baseURL)")
    failures << "#{path}: private HTTP client constructor must normalize Render hostport with config.ServiceURL"
  end
end

# Entrypoints with explicit production-only startup validation use Getenv so
# they can retain development fallbacks. Keep those requirements executable in
# the Blueprint gate instead of relying on a reviewer to notice the literals.
{
  "identity-service" => %w[ENVIRONMENT DATABASE_URL REDIS_URL NATS_URL INTERNAL_SERVICE_TOKEN MFA_ENCRYPTION_KEY],
  "tenant-service" => %w[DATABASE_URL INTERNAL_SERVICE_TOKEN],
  "ai-recommendation-service" => %w[ENVIRONMENT AI_REC_DATABASE_URL AI_REC_NATS_HOST AI_REC_STUDENT_SERVICE_URL AI_REC_TENANT_SERVICE_URL INTERNAL_SERVICE_TOKEN],
  "ai-prediction-service" => %w[ENVIRONMENT AI_PRED_DATABASE_URL AI_PRED_NATS_HOST AI_PRED_STUDENT_SERVICE_URL AI_PRED_TENANT_SERVICE_URL INTERNAL_SERVICE_TOKEN],
  "career-guidance-service" => %w[ENVIRONMENT AI_GUIDANCE_DATABASE_URL AI_GUIDANCE_NATS_HOST AI_GUIDANCE_STUDENT_SERVICE_URL AI_GUIDANCE_TENANT_SERVICE_URL INTERNAL_SERVICE_TOKEN],
}.each do |name, variables|
  service = services.fetch(name)
  env_vars = service.fetch("envVars", [])
  available = env_vars.map { |entry| entry["key"] }.compact
  env_vars.map { |entry| entry["fromGroup"] }.compact.each do |group|
    available.concat(groups.fetch(group, []))
  end
  variables.each do |variable|
    failures << "#{name}: missing production startup requirement #{variable}" unless available.include?(variable)
  end
end

identity_secret_consumers = services.each_with_object([]) do |(name, service), consumers|
  consumers << name if service.fetch("envVars", []).any? { |entry| entry["fromGroup"] == "auraedu-identity-secrets" }
end
unless identity_secret_consumers == ["identity-service"]
  failures << "auraedu-identity-secrets must be attached only to identity-service, got #{identity_secret_consumers.sort}"
end

identity_retention = {
  "AUTH_CLEANUP_INTERVAL" => "1h",
  "AUTH_CLEANUP_BATCH_SIZE" => "1000",
  "AUTH_REFRESH_RETENTION_AFTER_EXPIRY" => "24h",
  "AUTH_PASSWORD_RESET_RETENTION" => "720h",
  "AUTH_INVITE_RETENTION" => "2160h",
  "AUTH_PUBLISHED_OUTBOX_RETENTION" => "720h",
}
identity_worker_env = services.fetch("identity-worker").fetch("envVars", []).each_with_object({}) do |entry, vars|
  vars[entry["key"]] = entry if entry["key"]
end
identity_compose_env = compose_services.fetch("identity-worker").fetch("environment", {})
identity_retention.each do |key, expected|
  unless identity_worker_env.dig(key, "value").to_s == expected
    failures << "identity-worker: #{key} must be pinned to #{expected} in Render"
  end
  unless identity_compose_env[key].to_s == expected
    failures << "identity-worker: #{key} must match #{expected} in Compose"
  end
end

%w[ai-recommendation-service ai-prediction-service career-guidance-service].each do |name|
  unless services.fetch(name)["healthCheckPath"] == "/ready"
    failures << "#{name}: Render traffic health must use the database-backed /ready endpoint"
  end

  compose_service = compose_services.fetch(name)
  probe = compose_service.dig("healthcheck", "test")
  unless probe.is_a?(Array) && probe.join(" ").include?("/ready")
    failures << "#{name}: Compose health must use the database-backed /ready endpoint"
  end
  unless compose_services.fetch("api-gateway").dig("depends_on", name, "condition") == "service_healthy"
    failures << "api-gateway: Compose must wait for #{name} database readiness"
  end
end

ai_consumer_sources = {
  "ai-recommendation-service" => "apps/ai-recommendation-service/src/ai_recommendation_service/events/subscriber.py",
  "ai-prediction-service" => "apps/ai-prediction-service/src/ai_prediction_service/events/subscriber.py",
  "career-guidance-service" => "apps/career-guidance-service/src/career_guidance_service/events/subscriber.py",
}
ai_durable_prefixes = ai_consumer_sources.to_h do |name, path|
  prefix = File.read(path)[/^DURABLE_PREFIX = "([a-z0-9-]+)"$/, 1]
  failures << "#{name}: subscriber must declare a static DURABLE_PREFIX" unless prefix
  [name, prefix]
end
ai_durable_prefixes.compact!
if ai_durable_prefixes.values.uniq.length != ai_durable_prefixes.length
  failures << "AI subscriber durable prefixes must be unique per service: #{ai_durable_prefixes}"
end

# Every process using the live feature-entitlement gate must be connected to
# Tenant Service in both production and the local full-stack topology. This is
# intentionally derived from the entrypoints so a newly gated process cannot be
# added without its deployment wiring following it.
runtime_gate_deployments = Dir.glob("apps/*-service/cmd/**/*.go").each_with_object([]) do |path, deployments|
  next if path.end_with?("_test.go")
  next unless File.read(path).include?("flags.NewRuntimeGate(")

  app = path.split("/")[1]
  deployments << (path.include?("/cmd/worker/") ? app.sub(/-service\z/, "-worker") : app)
end.uniq.sort

runtime_gate_deployments.each do |name|
  service = services[name]
  unless service
    failures << "#{name}: live feature gate has no Render deployment"
    next
  end

  tenant_variables = service.fetch("envVars", []).select { |entry| entry["key"] == "SERVICE_TENANT_URL" }
  unless tenant_variables.length == 1
    failures << "#{name}: live feature gate requires exactly one Render SERVICE_TENANT_URL"
  else
    source = tenant_variables.first["fromService"]
    unless source == { "name" => "tenant-service", "property" => "hostport" }
      failures << "#{name}: Render SERVICE_TENANT_URL must use tenant-service hostport"
    end
  end

  compose_service = compose_services[name]
  unless compose_service
    failures << "#{name}: live feature gate has no Compose service"
    next
  end
  unless compose_service.fetch("environment", {})["SERVICE_TENANT_URL"] == "http://tenant-service:8082"
    failures << "#{name}: Compose SERVICE_TENANT_URL must target http://tenant-service:8082"
  end
end

payment = services.fetch("payment-service")
payment_env = payment.fetch("envVars", []).each_with_object({}) do |entry, vars|
  vars[entry["key"]] = entry if entry["key"]
end
unless payment_env.dig("PAYMENTS_PROVIDER", "value") == "paystack"
  failures << "payment-service: production PAYMENTS_PROVIDER must be pinned to paystack"
end
unless payment_env.dig("PAYSTACK_SECRET_KEY", "sync") == false
  failures << "payment-service: PAYSTACK_SECRET_KEY must be a secret-backed Render value"
end
if payment_env.key?("PAYSTACK_BASE_URL")
  failures << "payment-service: production must not override the canonical Paystack API origin"
end

twilio_callback = "https://auraedugh.vercel.app/api/v1/webhooks/twilio"
%w[notification-service notification-worker].each do |name|
  env = services.fetch(name).fetch("envVars", []).each_with_object({}) do |entry, vars|
    vars[entry["key"]] = entry if entry["key"]
  end
  unless env.dig("TWILIO_STATUS_CALLBACK_URL", "value") == twilio_callback
    failures << "#{name}: TWILIO_STATUS_CALLBACK_URL must use the stable Vercel relay"
  end
  compose_callback = compose_services.fetch(name).fetch("environment", {})["TWILIO_STATUS_CALLBACK_URL"].to_s
  unless compose_callback.include?(twilio_callback)
    failures << "#{name}: Compose TWILIO_STATUS_CALLBACK_URL must default to the stable Vercel relay"
  end
end

required.sort.each do |name, variables|
  service = services[name]
  unless service
    failures << "#{name}: no Render service exists"
    next
  end

  env_vars = service.fetch("envVars", [])
  available = env_vars.map { |entry| entry["key"] }.compact
  env_vars.map { |entry| entry["fromGroup"] }.compact.each do |group|
    available.concat(groups.fetch(group, []))
  end

  variables.sort.each do |variable, paths|
    next if available.include?(variable)

    failures << "#{name}: missing #{variable} (required by #{paths.join(', ')})"
  end
end

{
  "web" => "apps/web/Dockerfile",
  "marketing" => "apps/marketing/Dockerfile",
}.each do |name, dockerfile|
  failures << "#{name}: frontend must deploy to Vercel, not Render" if services.key?(name)

  docker = File.read(dockerfile)
  failures << "#{dockerfile}: pnpm must be pinned for Node 26 images" unless docker.include?("npm install --global pnpm@11.11.0")
  failures << "#{dockerfile}: Corepack is unavailable in Node 26" if docker.include?("corepack enable")
  install = docker.index("pnpm install --frozen-lockfile")
  if install.nil? || install > docker.index("COPY . .").to_i
    failures << "#{dockerfile}: dependency install must precede the full source copy for cache reuse"
  end
  expected_filter = name == "web" ? "--filter web..." : "--filter marketing..."
  failures << "#{dockerfile}: dependency install must use #{expected_filter}" unless docker.include?(expected_filter)
  failures << "#{dockerfile}: dependency downloads must use the shared BuildKit cache" unless docker.include?("id=auraedu-pnpm")
  runner = docker.split(/^FROM node:26\.5\.0-alpine(?:@sha256:[0-9a-f]{64})? AS runner$/, 2).last
  failures << "#{dockerfile}: runtime must use a clean Node stage" if runner == docker || runner.include?("pnpm")
  failures << "#{dockerfile}: Next telemetry must be disabled in build and runtime" unless docker.scan("ENV NEXT_TELEMETRY_DISABLED=1").length == 2
  %w[ENVIRONMENT NEXT_PUBLIC_API_URL NEXT_PUBLIC_APP_URL].each do |variable|
    failures << "#{dockerfile}: missing ARG #{variable}" unless docker.match?(/^ARG #{variable}(?:=|$)/)
    failures << "#{dockerfile}: missing ENV #{variable}" unless docker.match?(/^ENV #{variable}=\$#{variable}$/)
  end
  failures << "#{dockerfile}: frontend runtime must use USER node" unless docker.match?(/^USER node$/)

  vercel_path = "apps/#{name}/vercel.json"
  begin
    vercel = JSON.parse(File.read(vercel_path))
    failures << "#{vercel_path}: framework must be nextjs" unless vercel["framework"] == "nextjs"
    failures << "#{vercel_path}: install must run from the workspace root" unless vercel["installCommand"] == "cd ../.. && pnpm install --frozen-lockfile"
    expected_build = "cd ../.. && node tools/vercel/validate-environment.mjs #{name} && pnpm --filter #{name} build"
    failures << "#{vercel_path}: build must validate production origins and scope the app" unless vercel["buildCommand"] == expected_build
  rescue JSON::ParserError => error
    failures << "#{vercel_path}: invalid JSON (#{error.message})"
  end
end

unless failures.empty?
  warn "ERROR: Render hard-required runtime configuration is incomplete:"
  failures.each { |failure| warn "  - #{failure}" }
  exit 1
end

variable_count = required.values.sum(&:length) + identity_retention.length
puts "Render runtime configuration passed for #{variable_count} hard requirements across #{required.length} backend deployments and #{runtime_gate_deployments.length} live feature-gated processes; both frontends are Vercel-gated."
RUBY
