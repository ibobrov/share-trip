package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const TripPublishedEventName = "trip_published"

type SentNotificationTripPublishRequest struct {
	RequestID     string
	CorrelationID string
	CausationID   string
	Traceparent   string
	Tracestate    string

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
) (response *SentNotificationTripPublishResponse, err error) {
	ctx, span := otel.Tracer("sharetrip").Start(ctx, "OutboxInsert")
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "outbox insert failed")
		}
	}()
	tripPublishEventPayload, err := json.Marshal(entity.PublishTripPayload{
		RequestID: req.RequestID, CorrelationID: req.CorrelationID, CausationID: req.CausationID, Traceparent: req.Traceparent, Tracestate: req.Tracestate,
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

	span.SetAttributes(attribute.String("event_id", eventID.String()), attribute.String("trip_id", req.TripID.String()))
	logctx.Logger(ctx).Debug("outbox event inserted in transaction", "component", "TripUseCase", "operation", "OutboxInsert", "event_id", eventID, "causation_id", req.CausationID, "result", "pending_commit")
	return &SentNotificationTripPublishResponse{EventID: eventID}, nil
}
