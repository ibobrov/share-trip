package entity

import "github.com/google/uuid"

type PublishTripPayload struct {
	TripID uuid.UUID `json:"trip_id"`
}
