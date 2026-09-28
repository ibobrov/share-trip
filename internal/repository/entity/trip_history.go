package entity

import (
	"time"

	"github.com/google/uuid"
)

type TripHistory struct {
	ID         uuid.UUID
	TripID     uuid.UUID
	FromStatus *string
	ToStatus   string
	CreatedAt  time.Time
}
