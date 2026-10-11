package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

const TripPublishedEventName = "trip_published"

type SentNotificationTripPublishRequest struct {
	TripID uuid.UUID
}

func (u *TripUseCase) SentNotificationTripPublish(
	ctx context.Context,
	tx pgx.Tx,
	req SentNotificationTripPublishRequest,
) error {
	tripPublishEventPayload, err := json.Marshal(entity.PublishTripPayload{
		TripID: req.TripID,
	})
	if err != nil {
		return fmt.Errorf("marshal trip published payload: %w", err)
	}

	err = u.outboxRepo.Create(ctx, tx, entity.OutboxEvent{
		ID:          uuid.New(),
		EventName:   TripPublishedEventName,
		AggregateID: req.TripID,
		Payload:     tripPublishEventPayload,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		return fmt.Errorf("create trip published event: %w", err)
	}

	return nil
}
