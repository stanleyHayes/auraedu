# Custom-domain activation runbook

1. Confirm the tenant is active, entitled to `custom_domain`, and the requesting school administrator holds `features.manage`.
2. In School settings, create the domain challenge. Give the school the exact `_auraedu.<hostname>` TXT name and the once-visible verification value. Never copy the value into tickets or logs.
3. After DNS propagation, run **Check DNS now**. Do not continue unless state becomes `verified`.
4. Add the exact hostname to the production Render web/API edge according to the provider's current custom-domain procedure. Do not add wildcard custom origins.
5. Wait for Render to report certificate issuance and verify HTTPS from an independent network. Record the provider domain/certificate identifier.
6. As a platform administrator, submit that provider reference through the activation control. This atomically makes the hostname public and emits `tenant.custom_domain_activated.v1`.
7. Verify the hostname resolves to the intended tenant, the login/public site carries that tenant's branding, API preflight returns the exact origin, and an attacker suffix/HTTP/custom-port origin is rejected.
8. Store sanitized provider, DNS, HTTPS, routing and negative-CORS results below `release/evidence/records/AURA-9.5/`, hash them, and update the release manifest.

If certificate issuance, routing or tenant identity is wrong, do not activate. To remove an active hostname, first obtain the provider removal or incident reference, use the platform-only **Deactivate safely** control, prove the hostname no longer resolves to a tenant, and then finish provider cleanup. Never clear or replace it through the generic tenant patch endpoint.
