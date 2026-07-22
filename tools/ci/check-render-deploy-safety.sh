#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

ruby -ryaml <<'RUBY'
render = YAML.load_file('render.yaml')
services = render.fetch('services', [])
service_types = services.to_h { |service| [service.fetch('name'), service.fetch('type')] }
failures = []

gateway = services.find { |service| service['name'] == 'api-gateway' }
gateway_origins = gateway&.fetch('envVars', [])&.find { |variable| variable['key'] == 'GATEWAY_CORS_ORIGINS' }&.fetch('value', '')
if gateway_origins.to_s.empty? || gateway_origins.split(',').any? { |origin| origin.strip == '*' || !origin.strip.start_with?('https://') }
  failures << 'api-gateway: production CORS origins must be explicit HTTPS origins without wildcard-all'
end
gateway_proxy = gateway&.fetch('envVars', [])&.find { |variable| variable['key'] == 'GATEWAY_TRUSTED_PROXY' }&.fetch('value', '')
failures << 'api-gateway: production trusted proxy must be explicitly set to render' unless gateway_proxy == 'render'

services.each do |service|
  name = service.fetch('name')
  type = service.fetch('type')
  next if %w[keyvalue redis].include?(type)

  unless service['autoDeployTrigger'] == 'checksPass'
    failures << "#{name}: Git deploys must wait for linked CI checks (autoDeployTrigger: checksPass)"
  end

  if service['runtime'] == 'docker'
    failures << "#{name}: Docker services must use dockerCommand, not startCommand" if service.key?('startCommand')
    if type == 'worker' && service['dockerCommand'].to_s.empty?
      failures << "#{name}: Docker worker must define its process with dockerCommand"
    end

    paths = service.dig('buildFilter', 'paths')
    failures << "#{name}: Docker service must have a non-empty monorepo buildFilter.paths" unless paths.is_a?(Array) && !paths.empty?

    dockerfile = service['dockerfilePath']&.sub(%r{\A\./}, '')
    covers_dockerfile = paths.is_a?(Array) && paths.any? do |path|
      path == dockerfile || (path.end_with?('/**') && dockerfile&.start_with?(path.delete_suffix('**')))
    end
    if dockerfile && !covers_dockerfile
      failures << "#{name}: buildFilter.paths must include #{dockerfile}"
    end
  end

  env_keys = service.fetch('envVars', []).map { |variable| variable['key'] }.compact
  duplicate_keys = env_keys.group_by(&:itself).select { |_key, values| values.length > 1 }.keys
  failures << "#{name}: duplicate environment keys: #{duplicate_keys.sort.join(', ')}" unless duplicate_keys.empty?

  service.fetch('envVars', []).each do |variable|
    reference = variable['fromService']
    next unless reference

    target = reference['name']
    expected_type = service_types[target]
    failures << "#{name}/#{variable['key']}: fromService target #{target} does not exist" if expected_type.nil?
    unless reference['type'] == expected_type
      failures << "#{name}/#{variable['key']}: fromService type must be #{expected_type || 'the target service type'}"
    end
  end

  if type == 'web'
    health_path = service['healthCheckPath'].to_s
    failures << "#{name}: public web service must define an absolute healthCheckPath" unless health_path.start_with?('/') && health_path.length > 1
  elsif type == 'pserv' && service.key?('healthCheckPath')
    failures << "#{name}: Render private services do not accept healthCheckPath"
  end

  plan = service['plan'].to_s
  failures << "#{name}: production service plan must be explicit and non-free" if plan.empty? || plan == 'free'
  failures << "#{name}: production service region must be frankfurt" unless service['region'] == 'frankfurt'
end

unless failures.empty?
  warn 'ERROR: Render deploy-safety check failed:'
  failures.each { |failure| warn "  - #{failure}" }
  exit 1
end

puts "Render deploy safety passed for #{services.length - services.count { |service| %w[keyvalue redis].include?(service['type']) }} deployable services: provider-valid commands/references, CI-gated deploys, public health checks, scoped builds, paid plans and one production region."
RUBY
