package entity

import (
	"time"

	"github.com/google/uuid"
)

type Trip struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	FromPoint     string
	ToPoint       string
	DepartureTime time.Time
	Seats         int
	Status        string
	CreatedAt     time.Time
}
