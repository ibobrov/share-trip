package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
)

type GetTripRequest struct {
	TripID uuid.UUID
}

type GetTripResponse struct {
	ID             uuid.UUID `json:"id"`
	ClientID       uuid.UUID `json:"client_id"`
	FromPoint      string    `json:"from_point"`
	ToPoint        string    `json:"to_point"`
	DepartureTime  time.Time `json:"departure_time"`
	AvailableSeats int       `json:"available_seats"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func GetTripResponseFromTripDomain(t domain.Trip) GetTripResponse {
	return GetTripResponse{
		ID:             t.ID,
		ClientID:       t.ClientID,
		FromPoint:      t.FromPoint,
		ToPoint:        t.ToPoint,
		DepartureTime:  t.DepartureTime,
		AvailableSeats: t.Seats,
		Status:         string(t.Status),
		CreatedAt:      t.CreatedAt,
	}
}
