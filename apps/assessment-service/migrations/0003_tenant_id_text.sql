-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS assessments_tenant_isolation ON assessments;
DROP POLICY IF EXISTS scores_tenant_isolation ON scores;
ALTER TABLE scores DROP CONSTRAINT fk_scores_assessment;
ALTER TABLE assessments ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE scores ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE scores ADD CONSTRAINT fk_scores_assessment FOREIGN KEY (tenant_id,assessment_id) REFERENCES assessments(tenant_id,id);
CREATE POLICY assessments_tenant_isolation ON assessments USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY scores_tenant_isolation ON scores USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
