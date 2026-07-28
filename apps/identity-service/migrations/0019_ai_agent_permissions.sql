-- Grant school admins the AI agent permissions behind the /admin/automation UI:
-- configuring agents and independently approving controlled-action proposals.
-- +goose Up
UPDATE users
SET permissions = ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || ARRAY[
        'ai.agent.configure','ai.action.approve'
    ]::text[]) AS permission
)
WHERE role = 'school_admin';

-- +goose Down
UPDATE users
SET permissions = ARRAY(
    SELECT permission
    FROM unnest(permissions) AS permission
    WHERE permission <> ALL(ARRAY[
        'ai.agent.configure','ai.action.approve'
    ]::text[])
)
WHERE role = 'school_admin';
