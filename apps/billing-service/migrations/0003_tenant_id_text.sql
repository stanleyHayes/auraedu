-- +goose Up
-- +goose StatementBegin

-- Billing is scoped by tenant *code* (e.g. upshs), not a UUID — align with the
-- platform convention (identity VARCHAR(50), file-service 0004_tenant_id_text).
DROP POLICY IF EXISTS billing_subscriptions_isolation ON billing_subscriptions;
DROP POLICY IF EXISTS billing_invoices_isolation ON billing_invoices;

ALTER TABLE billing_subscriptions ALTER COLUMN tenant_id TYPE TEXT;
ALTER TABLE billing_invoices ALTER COLUMN tenant_id TYPE TEXT;

CREATE POLICY billing_subscriptions_isolation ON billing_subscriptions
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

CREATE POLICY billing_invoices_isolation ON billing_invoices
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS billing_subscriptions_isolation ON billing_subscriptions;
DROP POLICY IF EXISTS billing_invoices_isolation ON billing_invoices;

ALTER TABLE billing_subscriptions ALTER COLUMN tenant_id TYPE UUID USING tenant_id::uuid;
ALTER TABLE billing_invoices ALTER COLUMN tenant_id TYPE UUID USING tenant_id::uuid;

CREATE POLICY billing_subscriptions_isolation ON billing_subscriptions
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true);

CREATE POLICY billing_invoices_isolation ON billing_invoices
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose StatementEnd
