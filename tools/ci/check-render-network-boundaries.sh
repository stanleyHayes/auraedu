#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

ruby -ryaml <<'RUBY'
render = YAML.load_file('render.yaml')
databases = render.fetch('databases', [])
services = render.fetch('services', [])
failures = []

databases.each do |database|
  name = database.fetch('name', '<unnamed-database>')
  allowlist = database['ipAllowList']
  failures << "#{name}: ipAllowList must be an explicit empty list to deny public ingress" unless allowlist == []
end

key_values = services.select { |service| %w[keyvalue redis].include?(service['type']) }
key_values.each do |service|
  failures << "#{service.fetch('name')}: key-value public ingress must be denied" unless service['ipAllowList'] == []
end

expected_public = %w[api-gateway]
actual_public = services.select { |service| service['type'] == 'web' }.map { |service| service.fetch('name') }.sort
failures << "public web-service inventory changed: expected #{expected_public.join(', ')}, got #{actual_public.join(', ')}" unless actual_public == expected_public

nats = services.find { |service| service['name'] == 'nats' }
failures << 'nats must remain a private service (pserv)' unless nats && nats['type'] == 'pserv'

services.each do |service|
  name = service.fetch('name')
  type = service.fetch('type')
  if name.end_with?('-service') && type != 'pserv'
    failures << "#{name}: domain services must remain private services, got #{type}"
  elsif name.end_with?('-worker') && type != 'worker'
    failures << "#{name}: worker deployment must retain worker type, got #{type}"
  end

  service.fetch('envVars', []).each do |variable|
    database_ref = variable['fromDatabase']
    next unless database_ref

    unless database_ref['property'] == 'connectionString'
      failures << "#{name}/#{variable['key']}: database references must use Render's private connectionString"
    end
  end
end

unless failures.empty?
  warn 'ERROR: Render network boundary check failed:'
  failures.each { |failure| warn "  - #{failure}" }
  exit 1
end

puts "Render network boundaries passed: #{databases.length} private databases, #{key_values.length} private key-value store, #{services.count { |service| service['type'] == 'pserv' }} private services, and only #{expected_public.join(', ')} is a public web service; frontends are Vercel-owned."
RUBY
