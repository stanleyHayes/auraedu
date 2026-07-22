#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

ruby -ryaml <<'RUBY'
render = YAML.load_file("render.yaml")
services = render.fetch("services").to_h { |service| [service.fetch("name"), service] }

required = {}
Dir.glob("apps/*-service/**/*.{go,py}").sort.each do |path|
  next if path.include?("/tests/") || path.include?("/test_") || path.end_with?("_test.go")
  next unless File.read(path).include?("INTERNAL_SERVICE_TOKEN")

  app = path.split("/")[1]
  deployable = path.include?("/cmd/worker/") ? app.sub(/-service\z/, "-worker") : app
  required[deployable] ||= []
  required[deployable] << path
end

failures = []
required.sort.each do |name, sources|
  service = services[name]
  unless service
    failures << "#{name}: no Render service exists (referenced by #{sources.join(', ')})"
    next
  end

  env_vars = service.fetch("envVars", [])
  secret_group = env_vars.any? { |entry| entry["fromGroup"] == "auraedu-secrets" }
  unless secret_group
    failures << "#{name}: missing auraedu-secrets (INTERNAL_SERVICE_TOKEN used by #{sources.join(', ')})"
  end

  app = sources.first.split("/")[1]
  entrypoints = if name.end_with?("-worker")
                  Dir.glob("apps/#{app}/cmd/worker/*.go")
                elsif sources.any? { |path| path.end_with?(".py") }
                  Dir.glob("apps/#{app}/src/*/main.py")
                else
                  Dir.glob("apps/#{app}/cmd/server/*.go")
                end
  runtime = entrypoints.map { |path| File.read(path) }.join("\n")
  guard = runtime.include?('RequireProductionEnv("INTERNAL_SERVICE_TOKEN")') ||
          runtime.include?("validateProductionRuntime") ||
          runtime.include?("validate_production_runtime")
  unless guard
    failures << "#{name}: production startup does not require INTERNAL_SERVICE_TOKEN"
  end
end

unless failures.empty?
  warn "ERROR: Render private service-token wiring is incomplete:"
  failures.each { |failure| warn "  - #{failure}" }
  exit 1
end

puts "Render service-token wiring and fail-closed startup passed for #{required.length} server/worker deployments."
RUBY
