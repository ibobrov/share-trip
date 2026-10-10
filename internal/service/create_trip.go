package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/jackc/pgx/v5"
)

type CreateTripRequest struct {
	ClientID       uuid.UUID
	FromPoint      string
	ToPoint        string
	DepartureTime  time.Time
	AvailableSeats int
}

type CreateTripResponse struct {
	ID             uuid.UUID
	ClientID       uuid.UUID
	FromPoint      string
	ToPoint        string
	DepartureTime  time.Time
	AvailableSeats int
	Status         string
	CreatedAt      time.Time
}

func (s *TripService) CreateTrip(
	ctx context.Context,
	request CreateTripRequest,
) (CreateTripResponse, error) {

	logger := logctx.Logger(ctx).With(
		slog.String("service", "TripService"),
		slog.String("operation", "CreateTrip"),
		slog.String("client_id", request.ClientID.String()),
		slog.String("layer", "service"),
	)

	logger.Info("create trip started")

	started := time.Now()
	metricRsl := "success"

	defer func() {
		s.metrics.TripCreateTotal.WithLabelValues(metricRsl).Inc()
		s.metrics.TripCreateDuration.WithLabelValues(metricRsl).
			Observe(time.Since(started).Seconds())
	}()

	result, err := tx(ctx, s.pool, func(tx pgx.Tx) (*CreateTripResponse, error) {
		res, err := s.tripUseCase.CreateTrip(ctx, tx, domain.CreateTripRequest{
			ClientID:      request.ClientID,
			FromPoint:     request.FromPoint,
			ToPoint:       request.ToPoint,
			DepartureTime: request.DepartureTime,
			Seats:         request.AvailableSeats,
		})

		if err != nil {
			if errors.Is(err, domain.ErrDomain) {
				metricRsl = "validation_error"
			} else {
				metricRsl = "internal_error"
			}

			logger.Error(
				"create trip usecase failed",
				slog.String("layer", "transaction"),
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("usecase.CreateTrip: %w", err)
		}

		return &CreateTripResponse{
			ID:             res.ID,
			ClientID:       res.ClientID,
			FromPoint:      res.FromPoint,
			ToPoint:        res.ToPoint,
			DepartureTime:  res.DepartureTime,
			AvailableSeats: res.Seats,
			Status:         res.Status,
			CreatedAt:      res.CreatedAt,
		}, nil
	})

	if err != nil {
		logger.Error(
			"create trip failed",
			slog.Any("error", err),
		)
		return CreateTripResponse{}, fmt.Errorf("failed in transaction: %w", err)
	}

	logger.Info(
		"create trip completed",
		slog.String("trip_id", result.ID.String()),
	)

	return *result, nil
}
