-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS report_templates_tenant_isolation ON report_templates;
DROP POLICY IF EXISTS report_cards_tenant_isolation ON report_cards;
DROP POLICY IF EXISTS report_card_score_entries_tenant_isolation ON report_card_score_entries;
DROP POLICY IF EXISTS report_card_attendance_entries_tenant_isolation ON report_card_attendance_entries;
ALTER TABLE report_cards DROP CONSTRAINT fk_report_cards_template;
ALTER TABLE report_templates ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE report_cards ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE report_card_score_entries ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE report_card_attendance_entries ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE report_cards ADD CONSTRAINT fk_report_cards_template FOREIGN KEY (tenant_id,template_id) REFERENCES report_templates(tenant_id,id);
CREATE POLICY report_templates_tenant_isolation ON report_templates USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY report_cards_tenant_isolation ON report_cards USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY report_card_score_entries_tenant_isolation ON report_card_score_entries USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY report_card_attendance_entries_tenant_isolation ON report_card_attendance_entries USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
