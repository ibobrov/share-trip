package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

func (u *TripUseCase) GetTrip(
	ctx context.Context,
	tx pgx.Tx,
	req GetTripRequest,
) (response *GetTripResponse, err error) {
	tracer := otel.Tracer("sharetrip")

	ctx, span := tracer.Start(ctx, "TripUsecase.GetTrip")
	defer span.End()
	defer func() {
		if err != nil && !errors.Is(err, ErrNotFound) {
			span.RecordError(err)
			span.SetStatus(codes.Error, "operation failed")
		}
	}()

	trip, err := u.tripRepo.GetTripByID(ctx, tx, req.TripID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get trip: %w", err)
	}

	return &GetTripResponse{
		ID:             trip.ID,
		ClientID:       trip.ClientID,
		CompanyID:      trip.CompanyID,
		FromPoint:      trip.FromPoint,
		ToPoint:        trip.ToPoint,
		DepartureTime:  trip.DepartureTime,
		AvailableSeats: trip.Seats,
		Status:         trip.Status,
		CreatedAt:      trip.CreatedAt,
	}, nil
}
