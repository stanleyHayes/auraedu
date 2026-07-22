-- +goose Up
UPDATE users
SET permissions = ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || ARRAY[
        'campaign.read','campaign.create','campaign.update',
        'campaign.approve','campaign.publish','campaign.budget.approve'
    ]::text[]) AS permission
)
WHERE role = 'school_admin';

-- +goose Down
UPDATE users
SET permissions = ARRAY(
    SELECT permission
    FROM unnest(permissions) AS permission
    WHERE permission <> ALL(ARRAY[
        'campaign.read','campaign.create','campaign.update',
        'campaign.approve','campaign.publish','campaign.budget.approve'
    ]::text[])
)
WHERE role = 'school_admin';
