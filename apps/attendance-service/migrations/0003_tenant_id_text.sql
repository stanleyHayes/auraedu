-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS attendance_records_tenant_isolation ON attendance_records;
ALTER TABLE attendance_records ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
CREATE POLICY attendance_records_tenant_isolation ON attendance_records USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
