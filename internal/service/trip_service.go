package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TripService struct {
	pool        *pgxpool.Pool
	tripUseCase *domain.TripUseCase
}

func NewTripService(pool *pgxpool.Pool, tripUseCase *domain.TripUseCase) *TripService {
	return &TripService{
		pool:        pool,
		tripUseCase: tripUseCase,
	}
}

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

func (s *TripService) GetById(
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
