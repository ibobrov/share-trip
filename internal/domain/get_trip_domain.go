package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (u *TripUseCase) GetTrip(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (*Trip, error) {
	tripEntity, err := u.tripRepo.GetTripByID(ctx, tx, id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get trip: %w", err)
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
