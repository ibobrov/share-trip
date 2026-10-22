package service

import (
	"github.com/ibobrov/share_trip/internal/observability/metrics"
)

type TripService struct {
	pool           TransactionBeginner
	tripUseCase    TripUseCase
	metrics        *metrics.Metrics
	contractClient ContractChecker
	eventPublisher EventPublisher
}

func NewTripService(
	pool TransactionBeginner,
	tripUseCase TripUseCase,
	metrics *metrics.Metrics,
	contractClient ContractChecker,
	eventPublisher EventPublisher,
) *TripService {
	return &TripService{
		pool:           pool,
		tripUseCase:    tripUseCase,
		metrics:        metrics,
		contractClient: contractClient,
		eventPublisher: eventPublisher,
	}
}
