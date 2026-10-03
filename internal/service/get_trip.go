package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *TripService) GetTripById(
	ctx context.Context,
	id uuid.UUID,
) (domain.Trip, error) {
	result, err := tx(
		ctx,
		s.pool,
		func(tx pgx.Tx) (*domain.Trip, error) {
			return s.tripUseCase.GetTrip(ctx, tx, id)
		})

	if err != nil {
		return domain.Trip{}, err
	}

	return *result, nil
}
