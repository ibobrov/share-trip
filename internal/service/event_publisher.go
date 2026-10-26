package service

import (
	"context"
	"time"

	"github.com/google/uuid"
)

//go:generate $MOCKGEN -source=event_publisher.go -destination=mocks/event_publisher.gen.go -package=mocks

type TripPublishedEvent struct {
	RequestID     string
	CorrelationID string
	CausationID   string
	Traceparent   string
	Tracestate    string

	EventID    uuid.UUID
	TripID     uuid.UUID
	ClientID   uuid.UUID
	CompanyID  string
	OccurredAt time.Time
}

type EventPublisher interface {
	PublishTripPublished(ctx context.Context, event TripPublishedEvent) error
}
