package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
)

type CreateTripRequest struct {
	ClientID       string `json:"client_id" valid:"required,uuid"`
	FromPoint      string `json:"from_point" valid:"required,matches(\\S)"`
	ToPoint        string `json:"to_point" valid:"required,matches(\\S)"`
	DepartureTime  string `json:"departure_time" valid:"required,rfc3339"`
	AvailableSeats int    `json:"available_seats" valid:"required"`
}

type CreateTripResponse struct {
	ID             uuid.UUID `json:"id"`
	ClientID       uuid.UUID `json:"client_id"`
	FromPoint      string    `json:"from_point"`
	ToPoint        string    `json:"to_point"`
	DepartureTime  time.Time `json:"departure_time"`
	AvailableSeats int       `json:"available_seats"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func (r *CreateTripRequest) ToNewTripDomain() domain.NewTrip {
	clientID, _ := uuid.Parse(r.ClientID)
	departureTime, _ := time.Parse(time.RFC3339, r.DepartureTime)

	return domain.NewTrip{
		ClientID:      clientID,
		FromPoint:     r.FromPoint,
		ToPoint:       r.ToPoint,
		DepartureTime: departureTime,
		Seats:         r.AvailableSeats,
	}
}

func CreateTripResponseFromTripDomain(t domain.Trip) CreateTripResponse {
	return CreateTripResponse{
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
