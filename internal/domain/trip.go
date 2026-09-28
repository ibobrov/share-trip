package domain

import (
	"time"

	"github.com/google/uuid"
)

type TripStatus string

const (
	TripStatusDraft     TripStatus = "draft"
	TripStatusPublished TripStatus = "published"
	TripStatusCanceled  TripStatus = "canceled"
	TripStatusCompleted TripStatus = "completed"
)

type Trip struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	FromPoint     string
	ToPoint       string
	DepartureTime time.Time
	Seats         int
	Status        TripStatus
	CreatedAt     time.Time
}
