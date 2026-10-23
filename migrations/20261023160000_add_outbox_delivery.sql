-- +goose Up
ALTER TABLE outbox_event
    ADD COLUMN status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    ADD COLUMN attempts INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    ADD COLUMN last_error TEXT,
    ADD COLUMN sent_at TIMESTAMPTZ;

CREATE INDEX outbox_event_pending ON outbox_event (created_at, id) WHERE status = 'pending';

-- +goose Down
DROP INDEX outbox_event_pending;
ALTER TABLE outbox_event
    DROP COLUMN sent_at,
    DROP COLUMN last_error,
    DROP COLUMN attempts,
    DROP COLUMN status;
