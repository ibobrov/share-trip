package service

import (
	"context"

	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *TripService) CreateTrip(
	ctx context.Context,
	newTrip domain.NewTrip,
) (domain.Trip, error) {
	result, err := tx(
		ctx,
		s.pool,
		func(tx pgx.Tx) (*domain.Trip, error) {
			return s.tripUseCase.CreateTrip(ctx, tx, newTrip)
		})

	if err != nil {
		return domain.Trip{}, err
	}

	return *result, nil
}
