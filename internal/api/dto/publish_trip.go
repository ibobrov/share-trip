package dto

import (
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
)

type PublishTripRequest struct {
	TripID   string `json:"trip_id" valid:"required,uuid"`
	ClientID string `json:"client_id" valid:"required,uuid"`
}

type PublishTripResponse struct {
	TripID uuid.UUID `json:"trip_id"`
}

func (r *PublishTripRequest) ToPublishTripDomain() domain.PublishTripRequest {
	tripID, _ := uuid.Parse(r.TripID)
	clientID, _ := uuid.Parse(r.ClientID)

	return domain.PublishTripRequest{
		TripID:   tripID,
		ClientID: clientID,
	}
}
