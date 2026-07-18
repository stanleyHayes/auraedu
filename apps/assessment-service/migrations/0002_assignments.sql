-- +goose Up
-- +goose StatementBegin

-- AURA-14.9 Assignments: assignments are assessments with type='assignment'.
-- class_ids tags an assessment/assignment with the classes it targets and
-- published_at records when an assignment was published.

ALTER TABLE assessments ADD COLUMN IF NOT EXISTS class_ids UUID[] NOT NULL DEFAULT '{}';
ALTER TABLE assessments ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_assessments_class_ids ON assessments USING GIN (class_ids);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_assessments_class_ids;
ALTER TABLE assessments DROP COLUMN IF EXISTS published_at;
ALTER TABLE assessments DROP COLUMN IF EXISTS class_ids;

-- +goose StatementEnd
