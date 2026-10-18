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
	CompanyID      string
	FromPoint      string
	ToPoint        string
	DepartureTime  time.Time
	AvailableSeats int
}

type CreateTripResponse struct {
	ID             uuid.UUID
	ClientID       uuid.UUID
	CompanyID      string
	FromPoint      string
	ToPoint        string
	DepartureTime  time.Time
	AvailableSeats int
	Status         string
	CreatedAt      time.Time
}

func (s *TripService) CreateTrip(
	ctx context.Context,
	req CreateTripRequest,
) (CreateTripResponse, error) {

	logger := logctx.Logger(ctx).With(
		slog.String("service", "TripService"),
		slog.String("operation", "CreateTrip"),
		slog.String("client_id", req.ClientID.String()),
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

	option, err := s.contractClient.CheckOption(ctx, req.CompanyID, "trip_creation")
	if err != nil {
		metricRsl = "internal_error"
		return CreateTripResponse{}, fmt.Errorf("check trip_creation availability: %w", err)
	}
	if !option.Allowed {
		metricRsl = "forbidden"
		return CreateTripResponse{}, fmt.Errorf("%w: trip_creation is unavailable: %s", domain.ErrForbidden, option.Reason)
	}

	result, err := tx(ctx, s.pool, func(tx pgx.Tx) (*CreateTripResponse, error) {
		res, err := s.tripUseCase.CreateTrip(ctx, tx, domain.CreateTripRequest{
			ClientID:      req.ClientID,
			CompanyID:     req.CompanyID,
			FromPoint:     req.FromPoint,
			ToPoint:       req.ToPoint,
			DepartureTime: req.DepartureTime,
			Seats:         req.AvailableSeats,
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
			CompanyID:      res.CompanyID,
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
