package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type OutboxEvent struct {
	ID          uuid.UUID
	EventName   string
	AggregateID uuid.UUID
	Payload     json.RawMessage
	CreatedAt   time.Time
}
