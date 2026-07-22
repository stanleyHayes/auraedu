-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS cbt_questions_tenant_isolation ON cbt_questions;
DROP POLICY IF EXISTS cbt_exam_sessions_tenant_isolation ON cbt_exam_sessions;
DROP POLICY IF EXISTS cbt_submissions_tenant_isolation ON cbt_submissions;
ALTER TABLE cbt_submissions DROP CONSTRAINT fk_submissions_exam_session;
ALTER TABLE cbt_questions ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE cbt_exam_sessions ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE cbt_submissions ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE cbt_submissions ADD CONSTRAINT fk_submissions_exam_session FOREIGN KEY (tenant_id,exam_session_id) REFERENCES cbt_exam_sessions(tenant_id,id);
CREATE POLICY cbt_questions_tenant_isolation ON cbt_questions USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY cbt_exam_sessions_tenant_isolation ON cbt_exam_sessions USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY cbt_submissions_tenant_isolation ON cbt_submissions USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
