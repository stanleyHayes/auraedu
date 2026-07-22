-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS payments_tenant_isolation ON payments;
DROP POLICY IF EXISTS transactions_tenant_isolation ON transactions;
DROP POLICY IF EXISTS webhook_events_tenant_isolation ON webhook_events;
ALTER TABLE transactions DROP CONSTRAINT fk_transactions_payment;
ALTER TABLE payments ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE transactions ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE webhook_events ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE transactions ADD CONSTRAINT fk_transactions_payment FOREIGN KEY (tenant_id,payment_id) REFERENCES payments(tenant_id,id);
CREATE POLICY payments_tenant_isolation ON payments USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY transactions_tenant_isolation ON transactions USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY webhook_events_tenant_isolation ON webhook_events USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
