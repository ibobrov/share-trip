package service

import (
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TripService struct {
	pool        *pgxpool.Pool
	tripUseCase *domain.TripUseCase
	metrics     *metrics.Metrics
}

func NewTripService(
	pool *pgxpool.Pool,
	tripUseCase *domain.TripUseCase,
	metrics *metrics.Metrics,
) *TripService {
	return &TripService{
		pool:        pool,
		tripUseCase: tripUseCase,
		metrics:     metrics,
	}
}
