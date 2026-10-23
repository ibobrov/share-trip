package service

import (
	"github.com/ibobrov/share_trip/internal/observability/metrics"
)

type TripService struct {
	pool           TransactionBeginner
	tripUseCase    TripUseCase
	metrics        *metrics.Metrics
	contractClient ContractChecker
}

func NewTripService(
	pool TransactionBeginner,
	tripUseCase TripUseCase,
	metrics *metrics.Metrics,
	contractClient ContractChecker,
) *TripService {
	return &TripService{
		pool:           pool,
		tripUseCase:    tripUseCase,
		metrics:        metrics,
		contractClient: contractClient,
	}
}
