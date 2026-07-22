-- +goose Up
-- +goose StatementBegin
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('draft', 'pending', 'partial', 'paid', 'overdue', 'cancelled'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_tenant_id_id ON invoices (tenant_id, id);

CREATE TABLE receipts (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    invoice_id UUID NOT NULL,
    student_id UUID NOT NULL,
    payment_id UUID NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    applied_cents BIGINT NOT NULL CHECK (applied_cents >= 0 AND applied_cents <= amount_cents),
    overpayment_cents BIGINT NOT NULL CHECK (overpayment_cents = amount_cents - applied_cents),
    currency CHAR(3) NOT NULL,
    provider_reference TEXT,
    issued_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT receipts_invoice_fk FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices (tenant_id, id),
    CONSTRAINT receipts_payment_unique UNIQUE (tenant_id, payment_id)
);

CREATE INDEX idx_receipts_tenant_invoice ON receipts (tenant_id, invoice_id, issued_at DESC);
CREATE INDEX idx_receipts_tenant_student ON receipts (tenant_id, student_id, issued_at DESC);

ALTER TABLE receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE receipts FORCE ROW LEVEL SECURITY;
CREATE POLICY receipts_tenant_isolation ON receipts
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS receipts;
UPDATE invoices SET status = 'pending' WHERE status = 'partial';
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_status_check;
ALTER TABLE invoices ADD CONSTRAINT invoices_status_check
    CHECK (status IN ('draft', 'pending', 'paid', 'overdue', 'cancelled'));
DROP INDEX IF EXISTS idx_invoices_tenant_id_id;
-- +goose StatementEnd
