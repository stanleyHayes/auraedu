-- Grant the least-privilege AI workflow permissions needed by each existing role.
-- +goose Up
UPDATE users
SET permissions = ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || CASE role
        WHEN 'teacher' THEN ARRAY[
            'ai.view_recommendations', 'ai.approve_recommendations',
            'ai.view_predictions', 'ai.approve_predictions',
            'ai.view_guidance', 'ai.approve_guidance'
        ]::text[]
        WHEN 'student' THEN ARRAY['ai.view_recommendations', 'ai.view_guidance']::text[]
        WHEN 'parent' THEN ARRAY['ai.view_recommendations', 'ai.view_guidance']::text[]
        ELSE ARRAY[
            'ai.view_recommendations', 'ai.approve_recommendations',
            'ai.view_predictions', 'ai.approve_predictions',
            'ai.view_guidance', 'ai.approve_guidance'
        ]::text[]
    END) AS permission
)
WHERE role IN ('teacher', 'student', 'parent', 'school_admin', 'platform_super_admin');

-- +goose Down
UPDATE users
SET permissions = ARRAY(
    SELECT permission
    FROM unnest(permissions) AS permission
    WHERE permission <> ALL(ARRAY[
        'ai.view_recommendations', 'ai.approve_recommendations',
        'ai.view_predictions', 'ai.approve_predictions',
        'ai.view_guidance', 'ai.approve_guidance'
    ]::text[])
)
WHERE role IN ('teacher', 'student', 'parent', 'school_admin', 'platform_super_admin');
