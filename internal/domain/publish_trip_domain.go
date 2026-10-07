package domain

import (
	"context"
	"encoding/json"
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
		return nil, fmt.Errorf("%w: client %s is not driver of trip %s", ErrForbidden, req.ClientID, req.TripID)
	}

	// Повторный запрос успешен, но новых истории и события не создаёт.
	if trip.Status == string(TripStatusPublished) {
		return &trip.ID, ErrSkipOperation
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
		return nil, fmt.Errorf("CreateTripHistory: %w", err)
	}

	tripPublishEventPayload, err := json.Marshal(entity.PublishTripPayload{
		TripID: trip.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal trip published payload: %w", err)
	}

	err = u.outboxRepo.Create(ctx, tx, entity.OutboxEvent{
		ID:          uuid.New(),
		EventName:   "trip_published",
		AggregateID: trip.ID,
		Payload:     tripPublishEventPayload,
		CreatedAt:   createdAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create trip published event: %w", err)
	}

	return &trip.ID, nil
}
