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
	TripID     uuid.UUID
	ClientID   uuid.UUID
	CompanyID  string
	OccurredAt time.Time
}

type SentNotificationTripPublishResponse struct {
	EventID uuid.UUID
}

func (u *TripUseCase) SentNotificationTripPublish(
	ctx context.Context,
	tx pgx.Tx,
	req SentNotificationTripPublishRequest,
) (*SentNotificationTripPublishResponse, error) {
	tripPublishEventPayload, err := json.Marshal(entity.PublishTripPayload{
		TripID:    req.TripID,
		ClientID:  req.ClientID,
		CompanyID: req.CompanyID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal trip published payload: %w", err)
	}

	eventID := uuid.New()
	err = u.outboxRepo.Create(ctx, tx, entity.OutboxEvent{
		ID:          eventID,
		EventName:   TripPublishedEventName,
		AggregateID: req.TripID,
		Payload:     tripPublishEventPayload,
		CreatedAt:   req.OccurredAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create trip published event: %w", err)
	}

	return &SentNotificationTripPublishResponse{EventID: eventID}, nil
}
