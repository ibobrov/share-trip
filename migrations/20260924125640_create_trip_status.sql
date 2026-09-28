-- +goose Up
-- +goose StatementBegin
CREATE TYPE trip_status AS ENUM (
    'draft',
    'published',
    'canceled',
    'completed'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TYPE IF EXISTS trip_status;
-- +goose StatementEnd
