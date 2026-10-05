package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/jackc/pgx/v5"
)

func (s *TripService) CreateTrip(
	ctx context.Context,
	newTrip domain.NewTrip,
) (domain.Trip, error) {
	logger := logctx.Logger(ctx).With(
		slog.String("service", "TripService"),
		slog.String("operation", "CreateTrip"),
		slog.String("client_id", newTrip.ClientID.String()),
		slog.String("layer", "service"),
	)

	logger.Info("create trip started")

	result, err := tx(ctx, s.pool, func(tx pgx.Tx) (*domain.Trip, error) {
		trip, err := s.tripUseCase.CreateTrip(ctx, tx, newTrip)

		if err != nil {
			logger.Error(
				"create trip usecase failed",
				slog.String("layer", "transaction"),
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("usecase.CreateTrip: %w", err)
		}

		return trip, nil
	})

	if err != nil {
		logger.Error(
			"create trip failed",
			slog.Any("error", err),
		)
		return domain.Trip{}, fmt.Errorf("failed in transaction: %w", err)
	}

	logger.Info(
		"create trip completed",
		slog.String("trip_id", result.ID.String()),
	)

	return *result, nil
}
