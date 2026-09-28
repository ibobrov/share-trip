package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

func (u *TripUseCase) CreateTrip(
	ctx context.Context,
	tx pgx.Tx,
	newTrip NewTrip,
) (*Trip, error) {
	if newTrip.DepartureTime.Before(time.Now()) {
		return &Trip{}, IncorrectTripDepartureTime
	}
	if newTrip.Seats <= 0 {
		return &Trip{}, IncorrectTripSeats
	}

	tripEntity := entity.Trip{
		ID:            uuid.New(),
		ClientID:      newTrip.ClientID,
		FromPoint:     newTrip.FromPoint,
		ToPoint:       newTrip.ToPoint,
		DepartureTime: newTrip.DepartureTime,
		Seats:         newTrip.Seats,
		Status:        string(TripStatusDraft),
		CreatedAt:     time.Now().UTC(),
	}

	if err := u.tripRepo.Create(ctx, tx, tripEntity); err != nil {
		return nil, fmt.Errorf("create trip: %w", err)
	}

	if err := u.tripHistoryRepo.Create(ctx, tx, entity.TripHistory{
		ID:         uuid.New(),
		TripID:     tripEntity.ID,
		FromStatus: nil,
		ToStatus:   tripEntity.Status,
		CreatedAt:  tripEntity.CreatedAt,
	}); err != nil {
		return nil, fmt.Errorf("create trip history: %w", err)
	}

	return &Trip{
		ID:            tripEntity.ID,
		ClientID:      tripEntity.ClientID,
		FromPoint:     tripEntity.FromPoint,
		ToPoint:       tripEntity.ToPoint,
		DepartureTime: tripEntity.DepartureTime,
		Seats:         tripEntity.Seats,
		Status:        TripStatus(tripEntity.Status),
		CreatedAt:     tripEntity.CreatedAt,
	}, nil
}
