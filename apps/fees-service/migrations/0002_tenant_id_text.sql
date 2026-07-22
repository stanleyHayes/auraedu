-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS fee_structures_tenant_isolation ON fee_structures;
DROP POLICY IF EXISTS invoices_tenant_isolation ON invoices;
ALTER TABLE invoices DROP CONSTRAINT fk_invoices_fee_structure;
ALTER TABLE fee_structures ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE invoices ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE invoices ADD CONSTRAINT fk_invoices_fee_structure FOREIGN KEY (tenant_id,fee_structure_id) REFERENCES fee_structures(tenant_id,id);
CREATE POLICY fee_structures_tenant_isolation ON fee_structures USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY invoices_tenant_isolation ON invoices USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
