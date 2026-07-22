-- +goose Up
-- +goose StatementBegin
ALTER TABLE crm_leads
    ADD COLUMN score_confidence TEXT CHECK (score_confidence IN ('low','medium','high')),
    ADD COLUMN score_positive_factors JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN score_negative_factors JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN scored_at TIMESTAMPTZ;

CREATE TABLE crm_lead_score_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    lead_id UUID NOT NULL,
    score SMALLINT NOT NULL CHECK (score BETWEEN 0 AND 100),
    confidence TEXT NOT NULL CHECK (confidence IN ('low','medium','high')),
    positive_factors JSONB NOT NULL,
    negative_factors JSONB NOT NULL,
    rule_version TEXT NOT NULL,
    triggered_by TEXT NOT NULL,
    evaluated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT crm_score_lead_fk FOREIGN KEY (tenant_id,lead_id) REFERENCES crm_leads(tenant_id,id) ON DELETE CASCADE
);
CREATE INDEX crm_lead_score_history ON crm_lead_score_evaluations (tenant_id,lead_id,evaluated_at DESC);
ALTER TABLE crm_lead_score_evaluations ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_lead_score_evaluations FORCE ROW LEVEL SECURITY;
CREATE POLICY crm_lead_score_tenant_isolation ON crm_lead_score_evaluations
    USING (tenant_id=current_setting('app.tenant_id',true))
    WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS crm_lead_score_tenant_isolation ON crm_lead_score_evaluations;
DROP TABLE IF EXISTS crm_lead_score_evaluations;
ALTER TABLE crm_leads DROP COLUMN IF EXISTS scored_at, DROP COLUMN IF EXISTS score_negative_factors, DROP COLUMN IF EXISTS score_positive_factors, DROP COLUMN IF EXISTS score_confidence;
-- +goose StatementEnd
