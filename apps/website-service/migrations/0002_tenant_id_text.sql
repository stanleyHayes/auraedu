-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS website_pages_tenant_isolation ON website_pages;
DROP POLICY IF EXISTS website_sections_tenant_isolation ON website_sections;
ALTER TABLE website_sections DROP CONSTRAINT fk_website_sections_page;
ALTER TABLE website_pages ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE website_sections ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE website_sections ADD CONSTRAINT fk_website_sections_page FOREIGN KEY (tenant_id,page_id) REFERENCES website_pages(tenant_id,id) ON DELETE CASCADE;
CREATE POLICY website_pages_tenant_isolation ON website_pages USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY website_sections_tenant_isolation ON website_sections USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
