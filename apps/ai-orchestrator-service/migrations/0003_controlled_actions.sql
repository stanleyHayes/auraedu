-- +goose Up
-- +goose StatementBegin
CREATE TABLE ai_action_proposals (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    action TEXT NOT NULL,
    level SMALLINT NOT NULL CHECK (level BETWEEN 0 AND 4),
    policy_version TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    payload JSONB NOT NULL,
    payload_hash TEXT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending_approval','approved','executing','succeeded','failed','rejected','cancelled')),
    proposed_by TEXT NOT NULL,
    proposer_role TEXT NOT NULL,
    reviewed_by TEXT,
    reviewer_role TEXT,
    review_note TEXT,
    reviewed_at TIMESTAMPTZ,
    execution_attempts INTEGER NOT NULL DEFAULT 0,
    result JSONB,
    failure_code TEXT,
    failure_detail TEXT,
    executed_at TIMESTAMPTZ,
    idempotency_key_hash TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, idempotency_key_hash),
    UNIQUE (tenant_id, id)
);

CREATE INDEX ai_action_proposals_tenant_status_time ON ai_action_proposals (tenant_id, status, created_at DESC, id DESC);

CREATE TABLE ai_action_audit (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    action_id UUID NOT NULL REFERENCES ai_action_proposals(id),
    event TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_role TEXT NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX ai_action_audit_tenant_action_time ON ai_action_audit (tenant_id, action_id, occurred_at, id);

ALTER TABLE ai_action_proposals ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_action_proposals FORCE ROW LEVEL SECURITY;
CREATE POLICY ai_action_proposals_tenant_isolation ON ai_action_proposals
    USING (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true) = 'true')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true) = 'true');

ALTER TABLE ai_action_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_action_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY ai_action_audit_tenant_isolation ON ai_action_audit
    USING (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true) = 'true')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true) = 'true');

CREATE FUNCTION prevent_ai_action_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'ai_action_audit is append-only';
END;
$$;

CREATE TRIGGER ai_action_audit_no_update
    BEFORE UPDATE OR DELETE ON ai_action_audit
    FOR EACH ROW EXECUTE FUNCTION prevent_ai_action_audit_mutation();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS ai_action_audit_no_update ON ai_action_audit;
DROP FUNCTION IF EXISTS prevent_ai_action_audit_mutation;
DROP TABLE IF EXISTS ai_action_audit;
DROP TABLE IF EXISTS ai_action_proposals;
-- +goose StatementEnd
