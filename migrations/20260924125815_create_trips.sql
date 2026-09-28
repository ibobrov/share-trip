-- +goose Up
-- +goose StatementBegin
CREATE TABLE trips
(
    id             UUID PRIMARY KEY,
    client_id      UUID        NOT NULL,
    from_point     TEXT        NOT NULL,
    to_point       TEXT        NOT NULL,
    departure_time TIMESTAMPTZ NOT NULL,
    seats          INT         NOT NULL CHECK (seats > 0),
    status         trip_status NOT NULL DEFAULT 'draft',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trips;
-- +goose StatementEnd
