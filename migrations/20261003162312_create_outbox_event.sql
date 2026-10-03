-- +goose Up
-- +goose StatementBegin
CREATE TABLE outbox_event
(
    id           UUID PRIMARY KEY,
    event_name   TEXT        NOT NULL,
    aggregate_id UUID        NOT NULL,
    payload      JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS outbox_event;
-- +goose StatementEnd
