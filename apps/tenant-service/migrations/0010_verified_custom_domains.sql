-- +goose Up
CREATE TABLE tenant_custom_domains (
    tenant_code         TEXT PRIMARY KEY REFERENCES tenants(code) ON DELETE CASCADE,
    hostname            TEXT NOT NULL UNIQUE,
    status              TEXT NOT NULL CHECK (status IN ('pending_dns', 'verified', 'active', 'inactive')),
    txt_record_name     TEXT NOT NULL,
    challenge_hash      TEXT NOT NULL CHECK (length(challenge_hash) = 64),
    verified_at         TIMESTAMPTZ,
    activated_at        TIMESTAMPTZ,
    deactivated_at      TIMESTAMPTZ,
    provider_reference  TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tenant_custom_domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_custom_domains FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_custom_domains_tenant ON tenant_custom_domains
    USING (tenant_code = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_code = current_setting('app.tenant_id', true));

CREATE POLICY tenant_custom_domains_platform ON tenant_custom_domains
    USING (current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (current_setting('app.is_platform_admin', true)::boolean = true);

-- Existing unverified values were accepted by the old generic tenant update
-- path. Clear them rather than treating them as ownership or TLS evidence.
UPDATE tenants SET domain = NULL WHERE domain IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS tenant_custom_domains;
