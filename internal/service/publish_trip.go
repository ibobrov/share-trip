package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *TripService) PublishTrip(
	ctx context.Context,
	tripDomain domain.PublishTripRequest,
) (uuid.UUID, error) {
	result, err := tx(
		ctx,
		s.pool,
		func(tx pgx.Tx) (*uuid.UUID, error) {
			return s.tripUseCase.MoveTripDraftToPublish(ctx, tx, tripDomain)
		},
	)

	if err != nil {
		return uuid.Nil, err
	}

	if result == nil {
		return uuid.Nil, errors.New("publish trip returned nil UUID")
	}

	return *result, nil
}
