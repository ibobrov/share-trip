package domain

import "github.com/ibobrov/share_trip/internal/repository"

type TripUseCase struct {
	tripRepo        *repository.TripRepository
	tripHistoryRepo *repository.TripHistoryRepository
	outboxRepo      *repository.OutboxRepository
}

func NewTripUseCase(
	tripRepo *repository.TripRepository,
	tripHistoryRepo *repository.TripHistoryRepository,
	outboxRepo *repository.OutboxRepository,
) *TripUseCase {
	return &TripUseCase{
		tripRepo:        tripRepo,
		tripHistoryRepo: tripHistoryRepo,
		outboxRepo:      outboxRepo,
	}
}
