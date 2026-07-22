-- +goose Up
-- +goose StatementBegin
-- Onboarding decisions are retained for their configured privacy/audit period,
-- but must not prevent an authorized tenant deletion. Preserve the decision and
-- clear only the reference to the deleted tenant.
ALTER TABLE onboarding_requests
    DROP CONSTRAINT onboarding_requests_tenant_code_fkey;

ALTER TABLE onboarding_requests
    ADD CONSTRAINT onboarding_requests_tenant_code_fkey
    FOREIGN KEY (tenant_code) REFERENCES tenants(code) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE onboarding_requests
    DROP CONSTRAINT onboarding_requests_tenant_code_fkey;

ALTER TABLE onboarding_requests
    ADD CONSTRAINT onboarding_requests_tenant_code_fkey
    FOREIGN KEY (tenant_code) REFERENCES tenants(code);
-- +goose StatementEnd
