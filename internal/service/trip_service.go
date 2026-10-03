package service

import (
	"github.com/ibobrov/share_trip/internal/domain"
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
