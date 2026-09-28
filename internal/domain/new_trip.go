package domain

import (
	"time"

	"github.com/google/uuid"
)

type NewTrip struct {
	ClientID      uuid.UUID
	FromPoint     string
	ToPoint       string
	DepartureTime time.Time
	Seats         int
}
