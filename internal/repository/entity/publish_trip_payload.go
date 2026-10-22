package entity

import "github.com/google/uuid"

type PublishTripPayload struct {
	TripID    uuid.UUID `json:"trip_id"`
	ClientID  uuid.UUID `json:"client_id"`
	CompanyID string    `json:"company_id"`
}
