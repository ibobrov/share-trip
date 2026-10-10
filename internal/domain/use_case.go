package domain

import "github.com/ibobrov/share_trip/internal/repository"

type TripStatus string

const (
	TripStatusDraft     TripStatus = "draft"
	TripStatusPublished TripStatus = "published"
	TripStatusCanceled  TripStatus = "canceled"
	TripStatusCompleted TripStatus = "completed"
)

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
