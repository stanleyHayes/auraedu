-- +goose Up
-- +goose StatementBegin
CREATE TABLE competitor_summaries (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, period_from TIMESTAMPTZ NOT NULL, period_to TIMESTAMPTZ NOT NULL,
 status TEXT NOT NULL CHECK(status IN('pending_review','approved','rejected')),
 items JSONB NOT NULL, item_count INTEGER NOT NULL CHECK(item_count>=0), source_count INTEGER NOT NULL CHECK(source_count>=0),
 generated_by TEXT NOT NULL, reviewed_by TEXT, reviewed_at TIMESTAMPTZ, review_note TEXT,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 CHECK(period_to>period_from), UNIQUE(tenant_id,id)
);
CREATE INDEX competitor_summaries_tenant_period ON competitor_summaries(tenant_id,period_to DESC,id);
ALTER TABLE competitor_summaries ENABLE ROW LEVEL SECURITY; ALTER TABLE competitor_summaries FORCE ROW LEVEL SECURITY;
CREATE POLICY competitor_summaries_tenant ON competitor_summaries USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS competitor_summaries;
-- +goose StatementEnd
