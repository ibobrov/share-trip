-- +goose Up
-- +goose StatementBegin
CREATE TABLE trip_history
(
    id          UUID PRIMARY KEY,
    trip_id     UUID        NOT NULL REFERENCES trips (id) ON DELETE CASCADE,
    from_status trip_status,
    to_status   trip_status NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trip_history;
-- +goose StatementEnd
