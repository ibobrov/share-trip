package domain

import "github.com/ibobrov/share_trip/internal/repository"

type TripUseCase struct {
	tripRepo        *repository.TripRepository
	tripHistoryRepo *repository.TripHistoryRepository
}

func NewTripUseCase(tripRepo *repository.TripRepository, tripHistoryRepo *repository.TripHistoryRepository) *TripUseCase {
	return &TripUseCase{
		tripRepo:        tripRepo,
		tripHistoryRepo: tripHistoryRepo,
	}
}
