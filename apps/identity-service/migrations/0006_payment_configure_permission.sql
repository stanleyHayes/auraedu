-- Backfill the provider-configuration permission required by Payment Service
-- for webhook audit and administrative reconciliation endpoints.
-- +goose Up
UPDATE users
SET permissions = ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || ARRAY['payments.configure']::text[]) AS permission
)
WHERE role IN ('school_admin', 'platform_super_admin');

-- +goose Down
UPDATE users
SET permissions = ARRAY(
    SELECT permission
    FROM unnest(permissions) AS permission
    WHERE permission <> 'payments.configure'
)
WHERE role IN ('school_admin', 'platform_super_admin');
