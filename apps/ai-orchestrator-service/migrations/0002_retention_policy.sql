-- +goose Up
-- +goose StatementBegin
CREATE POLICY assistant_exchanges_platform_maintenance ON assistant_exchanges
    USING (current_setting('app.is_platform_admin', true) = 'true')
    WITH CHECK (current_setting('app.is_platform_admin', true) = 'true');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS assistant_exchanges_platform_maintenance ON assistant_exchanges;
-- +goose StatementEnd
