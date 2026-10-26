package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
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
) (response GetTripResponse, err error) {
	ctx, span := otel.Tracer("sharetrip").Start(ctx, "TripService.GetTrip")
	defer span.End()
	defer func() {
		if metricResult(err) == "internal_error" {
			span.RecordError(err)
			span.SetStatus(codes.Error, "operation failed")
		}
	}()

	result, err := tx(
		ctx,
		s.pool,
		func(tx pgx.Tx) (*GetTripResponse, error) {
			res, err := s.tripUseCase.GetTrip(ctx, tx, domain.GetTripRequest{TripID: req.TripID})

			if err != nil {
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
