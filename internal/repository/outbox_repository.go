package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

type OutboxRepository struct{}

func NewOutboxRepository() *OutboxRepository {
	return &OutboxRepository{}
}

func (r *OutboxRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	event entity.OutboxEvent,
) error {
	_, err := tx.Exec(
		ctx,
		`
			INSERT INTO outbox_event (
				id, event_name, aggregate_id, payload, created_at
			)
			VALUES ($1, $2, $3, $4, $5)
		`,
		event.ID,
		event.EventName,
		event.AggregateID,
		event.Payload,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}

// LockPending holds row locks until the caller commits or rolls back the transaction.
func (r *OutboxRepository) LockPending(ctx context.Context, tx pgx.Tx, limit int) ([]entity.OutboxEvent, error) {
	rows, err := tx.Query(
		ctx,
		`
 		SELECT id, event_name, aggregate_id, payload, created_at
 		FROM outbox_event WHERE status = 'pending'
 		ORDER BY created_at, id LIMIT $1 FOR UPDATE SKIP LOCKED
 		`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select pending outbox: %w", err)
	}
	defer rows.Close()
	var events []entity.OutboxEvent
	for rows.Next() {
		var event entity.OutboxEvent
		if err := rows.Scan(&event.ID, &event.EventName, &event.AggregateID, &event.Payload, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending outbox: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending outbox: %w", err)
	}
	return events, nil
}

func (r *OutboxRepository) MarkSent(ctx context.Context, tx pgx.Tx, id uuid.UUID, sentAt time.Time) error {
	result, err := tx.Exec(
		ctx,
		`
		UPDATE outbox_event SET status = 'sent', attempts = attempts + 1, last_error = NULL, sent_at = $2
 		WHERE id = $1 AND status = 'pending'
 		`,
		id,
		sentAt,
	)
	if err != nil {
		return fmt.Errorf("mark outbox sent: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("pending outbox event %s not found", id)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, tx pgx.Tx, id uuid.UUID, message string, maxAttempts int) error {
	result, err := tx.Exec(
		ctx,
		`
 		UPDATE outbox_event SET attempts = attempts + 1, last_error = $2,
		status = CASE WHEN attempts + 1 >= $3 THEN 'failed' ELSE 'pending' END
 		WHERE id = $1 AND status = 'pending'`,
		id,
		message,
		maxAttempts,
	)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("pending outbox event %s not found", id)
	}
	return nil
}
