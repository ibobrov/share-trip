package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type PublishTripRequest struct {
	TripID   uuid.UUID
	ClientID uuid.UUID
}

type PublishTripResponse struct {
	TripID      uuid.UUID
	ClientID    uuid.UUID
	CompanyID   string
	PublishedAt time.Time
}

func (u *TripUseCase) MoveTripDraftToPublish(
	ctx context.Context,
	tx pgx.Tx,
	req PublishTripRequest,
) (response *PublishTripResponse, err error) {
	tracer := otel.Tracer("sharetrip")

	ctx, span := tracer.Start(ctx, "TripUsecase.PublishTrip")
	defer span.End()
	defer func() {
		if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrForbidden) && !errors.Is(err, ErrConflict) && !errors.Is(err, ErrTripIsAlreadyPublished) {
			span.RecordError(err)
			span.SetStatus(codes.Error, "operation failed")
		}
	}()

	trip, err := u.tripRepo.GetTripForUpdateByID(ctx, tx, req.TripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: trip %s", ErrNotFound, req.TripID)
		}

		return nil, fmt.Errorf("tripRepository.GetForUpdateByID: %w", err)
	}

	if trip.ClientID != req.ClientID {
		return nil, fmt.Errorf("%w: client %s is not driver of trip %s", ErrForbidden, req.ClientID, req.TripID)
	}

	// Повторный запрос успешен, но новых истории и события не создаёт.
	if trip.Status == string(TripStatusPublished) {
		return nil, ErrTripIsAlreadyPublished
	}

	if trip.Status != string(TripStatusDraft) {
		return nil, fmt.Errorf("%w: invalid trip status: expected %s, got %s", ErrConflict, TripStatusDraft, trip.Status)
	}

	trip.Status = string(TripStatusPublished)
	createdAt := time.Now().UTC()

	ok, err := u.tripRepo.UpdateTrip(ctx, tx, trip)
	if err != nil {
		return nil, fmt.Errorf("update trip: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf(
			"update trip: no row updated for trip %s",
			trip.ID,
		)
	}

	err = u.tripHistoryRepo.CreateTripHistory(ctx, tx, entity.TripHistory{
		ID:         uuid.New(),
		TripID:     trip.ID,
		FromStatus: new(string(TripStatusDraft)),
		ToStatus:   string(TripStatusPublished),
		CreatedAt:  createdAt,
	})
	if err != nil {
		return nil, fmt.Errorf("tripHistoryRepo.CreateTripHistory: %w", err)
	}

	return &PublishTripResponse{
		TripID: trip.ID, ClientID: trip.ClientID, CompanyID: trip.CompanyID, PublishedAt: createdAt,
	}, nil
}
