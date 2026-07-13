-- +goose Up
-- +goose StatementBegin

-- Staff is scoped by tenant *code* (e.g. upshs), not a UUID — align with the
-- platform convention (identity VARCHAR(50), file-service 0004_tenant_id_text).
DROP POLICY IF EXISTS staff_tenant_isolation ON staff;

ALTER TABLE staff ALTER COLUMN tenant_id TYPE TEXT;

CREATE POLICY staff_tenant_isolation ON staff
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS staff_tenant_isolation ON staff;

ALTER TABLE staff ALTER COLUMN tenant_id TYPE UUID USING tenant_id::uuid;

CREATE POLICY staff_tenant_isolation ON staff
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose StatementEnd
