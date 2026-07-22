-- Backfill Growth permissions for existing school administrators. Feature flags
-- remain the entitlement boundary, so assigning permission never enables a module.
-- +goose Up
UPDATE users
SET permissions = ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || ARRAY[
        'crm.lead.read','crm.lead.create','crm.lead.update','crm.lead.assign','crm.lead.export','crm.interaction.create',
        'knowledge.read','knowledge.manage','knowledge.approve','feedback.review','analytics.executive.read'
    ]::text[]) AS permission
)
WHERE role = 'school_admin';

-- +goose Down
UPDATE users
SET permissions = ARRAY(
    SELECT permission
    FROM unnest(permissions) AS permission
    WHERE permission <> ALL(ARRAY[
        'crm.lead.read','crm.lead.create','crm.lead.update','crm.lead.assign','crm.lead.export','crm.interaction.create',
        'knowledge.read','knowledge.manage','knowledge.approve','feedback.review','analytics.executive.read'
    ]::text[])
)
WHERE role = 'school_admin';
