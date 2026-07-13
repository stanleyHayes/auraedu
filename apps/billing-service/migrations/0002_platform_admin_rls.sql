-- +goose Up
-- Platform super admins may read/modify billing subscriptions and invoices across tenants.

DROP POLICY IF EXISTS billing_subscriptions_isolation ON billing_subscriptions;
CREATE POLICY billing_subscriptions_isolation ON billing_subscriptions
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.is_platform_admin', true)::boolean = true);

DROP POLICY IF EXISTS billing_invoices_isolation ON billing_invoices;
CREATE POLICY billing_invoices_isolation ON billing_invoices
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose Down
DROP POLICY IF EXISTS billing_invoices_isolation ON billing_invoices;
CREATE POLICY billing_invoices_isolation ON billing_invoices
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS billing_subscriptions_isolation ON billing_subscriptions;
CREATE POLICY billing_subscriptions_isolation ON billing_subscriptions
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);
