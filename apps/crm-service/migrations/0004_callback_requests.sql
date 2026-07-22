-- +goose Up
-- +goose StatementBegin
CREATE TABLE crm_callback_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    lead_id UUID NOT NULL,
    preferred_at TIMESTAMPTZ NOT NULL,
    timezone TEXT NOT NULL,
    locale TEXT NOT NULL CHECK (locale IN ('en', 'en-GH', 'fr', 'fr-GH')),
    status TEXT NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'confirmed', 'completed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT crm_callback_requests_lead_fk FOREIGN KEY (tenant_id, lead_id)
        REFERENCES crm_leads (tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, id)
);

CREATE INDEX crm_callback_requests_tenant_status_time
    ON crm_callback_requests (tenant_id, status, preferred_at, id);

ALTER TABLE crm_callback_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_callback_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY crm_callback_requests_tenant_isolation ON crm_callback_requests
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS crm_callback_requests;
-- +goose StatementEnd
