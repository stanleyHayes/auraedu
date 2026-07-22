-- +goose Up
-- +goose StatementBegin
ALTER TABLE crm_interactions
    ADD CONSTRAINT crm_interactions_tenant_id_unique UNIQUE (tenant_id, id);

CREATE TABLE crm_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    interaction_id UUID,
    ai_run_id UUID,
    feedback_type TEXT NOT NULL CHECK (feedback_type IN ('helpful','unhelpful','incorrect','outdated','unresolved','escalation_requested')),
    rating SMALLINT CHECK (rating BETWEEN 1 AND 5),
    comment TEXT CHECK (char_length(comment) <= 2000),
    review_status TEXT NOT NULL DEFAULT 'pending' CHECK (review_status IN ('pending','reviewed','dismissed')),
    reviewed_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ,
    CONSTRAINT crm_feedback_interaction_fk FOREIGN KEY (tenant_id, interaction_id)
        REFERENCES crm_interactions (tenant_id, id) ON DELETE SET NULL (interaction_id)
);
CREATE INDEX crm_feedback_tenant_review_created ON crm_feedback (tenant_id, review_status, created_at DESC, id);

ALTER TABLE crm_feedback ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_feedback FORCE ROW LEVEL SECURITY;
CREATE POLICY crm_feedback_tenant_isolation ON crm_feedback
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS crm_feedback;
ALTER TABLE crm_interactions DROP CONSTRAINT IF EXISTS crm_interactions_tenant_id_unique;
-- +goose StatementEnd
