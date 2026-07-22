-- +goose Up
-- Existing timestamp values were written as UTC. Convert them without applying
-- the database session timezone, and remain safe when another AI service has
-- already upgraded the shared feature-store table.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'feature_store_metrics'
          AND column_name = 'recorded_at'
          AND data_type = 'timestamp without time zone'
    ) THEN
        ALTER TABLE feature_store_metrics
            ALTER COLUMN recorded_at TYPE timestamptz USING recorded_at AT TIME ZONE 'UTC',
            ALTER COLUMN created_at TYPE timestamptz USING created_at AT TIME ZONE 'UTC';
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'recommendations'
          AND column_name = 'created_at'
          AND data_type = 'timestamp without time zone'
    ) THEN
        ALTER TABLE recommendations
            ALTER COLUMN approved_at TYPE timestamptz USING approved_at AT TIME ZONE 'UTC',
            ALTER COLUMN created_at TYPE timestamptz USING created_at AT TIME ZONE 'UTC',
            ALTER COLUMN updated_at TYPE timestamptz USING updated_at AT TIME ZONE 'UTC';
    END IF;
END $$;
