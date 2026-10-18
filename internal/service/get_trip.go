package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
)

type GetTripRequest struct {
	TripID uuid.UUID
}

type GetTripResponse struct {
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

func (s *TripService) GetTripByIdRequest(
	ctx context.Context,
	req GetTripRequest,
) (GetTripResponse, error) {
	ctx, span := otel.Tracer("TripService").Start(ctx, "TripService.GetTrip")
	defer span.End()

	logger := logctx.Logger(ctx).With(
		slog.String("service", "TripService"),
		slog.String("operation", "GetTrip"),
		slog.String("layer", "service"),
	)

	result, err := tx(
		ctx,
		s.pool,
		func(tx pgx.Tx) (*GetTripResponse, error) {
			res, err := s.tripUseCase.GetTrip(ctx, tx, domain.GetTripRequest{TripID: req.TripID})

			if err != nil {

				logger.Error(
					"get trip usecase failed",
					slog.String("layer", "transaction"),
					slog.Any("error", err),
				)
				return nil, fmt.Errorf("usecase.GetTrip: %w", err)
			}

			return &GetTripResponse{
				ID:             res.ID,
				ClientID:       res.ClientID,
				CompanyID:      res.CompanyID,
				FromPoint:      res.FromPoint,
				ToPoint:        res.ToPoint,
				DepartureTime:  res.DepartureTime,
				AvailableSeats: res.AvailableSeats,
				Status:         res.Status,
				CreatedAt:      res.CreatedAt,
			}, nil
		})

	if err != nil {
		return GetTripResponse{}, err
	}

	return *result, nil
}
