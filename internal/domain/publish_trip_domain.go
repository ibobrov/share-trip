package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

func (u *TripUseCase) MoveTripDraftToPublish(
	ctx context.Context,
	tx pgx.Tx,
	req PublishTripRequest,
) (*uuid.UUID, error) {
	trip, err := u.tripRepo.GetForUpdateByID(ctx, tx, req.TripID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: trip %s", ErrNotFound, req.TripID)
		}

		return nil, fmt.Errorf("tripRepository.GetForUpdateByID: %w", err)
	}

	if trip.ClientID != req.ClientID {
		return nil, fmt.Errorf("%w: forbidden: client %s is not driver of trip %s", ErrDomain, req.ClientID, req.TripID)
	}

	if trip.Status == string(TripStatusPublished) {
		return &trip.ID, nil
	}

	if trip.Status != string(TripStatusDraft) {
		return nil, fmt.Errorf("%w: invalid trip status: expected %s, got %s", ErrDomain, TripStatusDraft, trip.Status)
	}

	trip.Status = string(TripStatusPublished)

	ok, err := u.tripRepo.UpdateTrip(ctx, tx, trip)
	if !ok {
		return nil, err
	}

	fromStatus := string(TripStatusDraft)

	err = u.tripHistoryRepo.CreateTripHistory(ctx, tx, entity.TripHistory{
		ID:         uuid.New(),
		TripID:     trip.ID,
		FromStatus: &fromStatus,
		ToStatus:   string(TripStatusPublished),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("CreateTripHistory: %w", err)
	}

	return &trip.ID, nil
}
