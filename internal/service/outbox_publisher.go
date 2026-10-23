package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

type OutboxStore interface {
	LockPending(context.Context, pgx.Tx, int) ([]entity.OutboxEvent, error)
	MarkSent(context.Context, pgx.Tx, uuid.UUID, time.Time) error
	MarkFailed(context.Context, pgx.Tx, uuid.UUID, string) error
}

// OutboxPublisher is an infrastructure worker; it does not change trip business state.
type OutboxPublisher struct {
	pool     TransactionBeginner
	outbox   OutboxStore
	producer EventPublisher
	logger   *slog.Logger
}

func NewOutboxPublisher(pool TransactionBeginner, outbox OutboxStore, producer EventPublisher, logger *slog.Logger) *OutboxPublisher {
	return &OutboxPublisher{pool: pool, outbox: outbox, producer: producer, logger: logger}
}

type PublishPendingRequest struct {
	Limit int
}

type PublishPendingResponse struct {
	Sent   int
	Failed int
}

func (p *OutboxPublisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			batchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			_, err := p.PublishPending(batchCtx, PublishPendingRequest{Limit: 10})
			cancel()
			if err != nil && ctx.Err() == nil {
				p.logger.Error("publish outbox batch", "error", err)
			}
		}
	}
}

func (p *OutboxPublisher) PublishPending(ctx context.Context, req PublishPendingRequest) (PublishPendingResponse, error) {
	if req.Limit <= 0 {
		return PublishPendingResponse{}, fmt.Errorf("outbox batch limit must be positive")
	}
	result, err := tx(ctx, p.pool, func(writeTx pgx.Tx) (*PublishPendingResponse, error) {
		pending, err := p.outbox.LockPending(ctx, writeTx, req.Limit)
		if err != nil {
			return nil, fmt.Errorf("lock pending outbox: %w", err)
		}
		response := &PublishPendingResponse{}
		for _, event := range pending {
			if err := p.publish(ctx, event); err != nil {
				if markErr := p.outbox.MarkFailed(ctx, writeTx, event.ID, err.Error()); markErr != nil {
					return nil, fmt.Errorf("mark outbox failure: %w", markErr)
				}
				p.logger.Error("publish outbox event", "event_id", event.ID, "error", err)
				response.Failed++
				continue
			}
			if err := p.outbox.MarkSent(ctx, writeTx, event.ID, time.Now().UTC()); err != nil {
				return nil, fmt.Errorf("mark outbox sent: %w", err)
			}
			response.Sent++
		}
		return response, nil
	})
	if err != nil {
		return PublishPendingResponse{}, err
	}
	return *result, nil
}

func (p *OutboxPublisher) publish(ctx context.Context, event entity.OutboxEvent) error {
	if event.EventName != domain.TripPublishedEventName {
		return fmt.Errorf("unsupported outbox event: %s", event.EventName)
	}
	var payload struct {
		TripID    uuid.UUID `json:"trip_id"`
		ClientID  uuid.UUID `json:"client_id"`
		CompanyID string    `json:"company_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode outbox TripPublished: %w", err)
	}
	if payload.TripID != event.AggregateID || payload.TripID == uuid.Nil || payload.ClientID == uuid.Nil || payload.CompanyID == "" {
		return fmt.Errorf("invalid outbox TripPublished identifiers")
	}
	return p.producer.PublishTripPublished(ctx, TripPublishedEvent{
		EventID: event.ID, TripID: payload.TripID, ClientID: payload.ClientID,
		CompanyID: payload.CompanyID, OccurredAt: event.CreatedAt.UTC(),
	})
}
