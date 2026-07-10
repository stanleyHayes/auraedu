-- +goose Up
-- +goose StatementBegin

-- Payment Service schema (EP-17): Payment, Transaction and WebhookEvent aggregates.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS payments (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          UUID NOT NULL,
    invoice_id         UUID NOT NULL,
    amount_cents       INTEGER NOT NULL CHECK (amount_cents > 0),
    currency           VARCHAR(3) NOT NULL DEFAULT 'GHS',
    provider           VARCHAR(20) NOT NULL CHECK (provider IN ('paystack', 'flutterwave', 'mock')),
    provider_reference TEXT,
    status             VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'success', 'failed', 'cancelled')),
    metadata           JSONB NOT NULL DEFAULT '{}'::jsonb,
    initiated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_tenant_id_id ON payments (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_payments_tenant_id ON payments (tenant_id);
CREATE INDEX IF NOT EXISTS idx_payments_invoice_id ON payments (invoice_id);
CREATE INDEX IF NOT EXISTS idx_payments_provider ON payments (provider);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments (status);
CREATE INDEX IF NOT EXISTS idx_payments_provider_reference ON payments (tenant_id, provider, provider_reference);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments (created_at, id);

CREATE TABLE IF NOT EXISTS transactions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    payment_id   UUID NOT NULL,
    type         VARCHAR(20) NOT NULL CHECK (type IN ('debit', 'credit', 'refund')),
    status       VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failed')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
    reference    TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_transactions_payment
        FOREIGN KEY (tenant_id, payment_id)
        REFERENCES payments (tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_tenant_id_id ON transactions (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_transactions_tenant_id ON transactions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_transactions_payment_id ON transactions (payment_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions (created_at, id);

CREATE TABLE IF NOT EXISTS webhook_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID,
    provider     VARCHAR(20) NOT NULL,
    event_type   VARCHAR(100) NOT NULL,
    payload      JSONB NOT NULL,
    signature    TEXT,
    processed    BOOLEAN NOT NULL DEFAULT false,
    processed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_events_tenant_id_id ON webhook_events (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_webhook_events_tenant_id ON webhook_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_webhook_events_provider ON webhook_events (provider);
CREATE INDEX IF NOT EXISTS idx_webhook_events_event_type ON webhook_events (event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_events_processed ON webhook_events (processed);
CREATE INDEX IF NOT EXISTS idx_webhook_events_created_at ON webhook_events (created_at, id);

ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments FORCE ROW LEVEL SECURITY;
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS payments_tenant_isolation ON payments;
CREATE POLICY payments_tenant_isolation ON payments
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS transactions_tenant_isolation ON transactions;
CREATE POLICY transactions_tenant_isolation ON transactions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS webhook_events_tenant_isolation ON webhook_events;
CREATE POLICY webhook_events_tenant_isolation ON webhook_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS webhook_events_tenant_isolation ON webhook_events;
DROP POLICY IF EXISTS transactions_tenant_isolation ON transactions;
DROP POLICY IF EXISTS payments_tenant_isolation ON payments;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS payments;

-- +goose StatementEnd
