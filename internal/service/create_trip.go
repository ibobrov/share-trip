package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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
) (response CreateTripResponse, err error) {
	ctx, span := otel.Tracer("sharetrip").Start(ctx, "TripService.CreateTrip")
	defer span.End()
	started := time.Now()
	defer func() {
		result := metricResult(err)
		if result == "internal_error" {
			span.RecordError(err)
			span.SetStatus(codes.Error, "operation failed")
		}
		logctx.Logger(ctx).Debug("trip creation completed", "component", "TripService", "operation", "CreateTrip", "trip_id", response.ID, "result", result)
		s.metrics.TripCreateTotal.WithLabelValues(result).Inc()
		s.metrics.TripCreateDuration.WithLabelValues(result).Observe(time.Since(started).Seconds())
	}()

	option, err := s.contractClient.CheckOption(ctx, req.CompanyID, "trip_creation")
	if err != nil {
		return CreateTripResponse{}, fmt.Errorf("check trip_creation availability: %w", err)
	}
	if !option.Allowed {
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
		return CreateTripResponse{}, fmt.Errorf("failed in transaction: %w", err)
	}
	return *result, nil
}
