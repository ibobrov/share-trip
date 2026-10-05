package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

func (u *TripUseCase) CreateTrip(
	ctx context.Context,
	tx pgx.Tx,
	newTrip NewTrip,
) (*Trip, error) {
	logger := logctx.Logger(ctx).With(
		slog.String("layer", "usecase"),
		slog.String("usecase", "TripUsecase.CreateTrip"),
		slog.String("client_id", newTrip.ClientID.String()),
	)
	logger.Info("create trip usecase started")

	if newTrip.DepartureTime.Before(time.Now()) {
		return &Trip{}, fmt.Errorf("%w: недопустимое время начало поездки", ErrDomain)
	}
	if newTrip.Seats <= 0 {
		return &Trip{}, fmt.Errorf("%w: недопустимое кол-во слотов в поездке", ErrDomain)
	}

	tripEntity := entity.Trip{
		ID:            uuid.New(),
		ClientID:      newTrip.ClientID,
		FromPoint:     newTrip.FromPoint,
		ToPoint:       newTrip.ToPoint,
		DepartureTime: newTrip.DepartureTime,
		Seats:         newTrip.Seats,
		Status:        string(TripStatusDraft),
		CreatedAt:     time.Now().UTC(),
	}

	if err := u.tripRepo.CreateTrip(ctx, tx, tripEntity); err != nil {
		logger.Error(
			"repository create trip failed",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("tripRepo.CreateTrip: %w", err)
	}

	if err := u.tripHistoryRepo.CreateTripHistory(ctx, tx, entity.TripHistory{
		ID:         uuid.New(),
		TripID:     tripEntity.ID,
		FromStatus: nil,
		ToStatus:   tripEntity.Status,
		CreatedAt:  tripEntity.CreatedAt,
	}); err != nil {
		logger.Error(
			"repository create trip history failed",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("tripHistoryRepo.CreateTripHistory: %w", err)
	}

	logger.Info(
		"create trip usecase completed",
		slog.String("trip_id", tripEntity.ID.String()),
	)

	return &Trip{
		ID:            tripEntity.ID,
		ClientID:      tripEntity.ClientID,
		FromPoint:     tripEntity.FromPoint,
		ToPoint:       tripEntity.ToPoint,
		DepartureTime: tripEntity.DepartureTime,
		Seats:         tripEntity.Seats,
		Status:        TripStatus(tripEntity.Status),
		CreatedAt:     tripEntity.CreatedAt,
	}, nil
}
