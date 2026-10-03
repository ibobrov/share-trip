package repository

import (
	"context"
	"fmt"

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
