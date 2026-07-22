-- +goose Up
-- +goose StatementBegin
WITH ranked_trials AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant_id ORDER BY created_at, id) AS trial_rank
    FROM billing_subscriptions
    WHERE status = 'trialing'
)
UPDATE billing_subscriptions AS subscription
SET status = 'cancelled', cancelled_at = COALESCE(cancelled_at, now()), updated_at = now()
FROM ranked_trials
WHERE subscription.id = ranked_trials.id AND ranked_trials.trial_rank > 1;

CREATE UNIQUE INDEX billing_subscriptions_single_trial_per_tenant
    ON billing_subscriptions (tenant_id)
    WHERE status = 'trialing';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS billing_subscriptions_single_trial_per_tenant;
-- +goose StatementEnd
