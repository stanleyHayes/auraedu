-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE timetable_entries (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    term_id UUID NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE RESTRICT,
    teacher_id UUID,
    weekday SMALLINT NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    start_minute SMALLINT NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute SMALLINT NOT NULL CHECK (end_minute BETWEEN 1 AND 1440 AND end_minute > start_minute),
    room TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_timetable_tenant_class_day ON timetable_entries (tenant_id, class_id, weekday, start_minute);
ALTER TABLE timetable_entries ADD CONSTRAINT timetable_class_no_overlap EXCLUDE USING gist (tenant_id WITH =, class_id WITH =, term_id WITH =, weekday WITH =, int4range(start_minute, end_minute, '[)') WITH &&) WHERE (status = 'active');
ALTER TABLE timetable_entries ADD CONSTRAINT timetable_teacher_no_overlap EXCLUDE USING gist (tenant_id WITH =, teacher_id WITH =, term_id WITH =, weekday WITH =, int4range(start_minute, end_minute, '[)') WITH &&) WHERE (status = 'active' AND teacher_id IS NOT NULL);
ALTER TABLE timetable_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE timetable_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY timetable_tenant_isolation ON timetable_entries USING (tenant_id = current_setting('app.tenant_id', true)) WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS timetable_entries;
-- +goose StatementEnd
