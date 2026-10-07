package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
)

func (s *TripService) PublishTrip(
	ctx context.Context,
	tripDomain domain.PublishTripRequest,
) (uuid.UUID, error) {
	ctx, span := otel.Tracer("TripService").Start(ctx, "TripService.PublishTrip")
	defer span.End()

	started := time.Now()
	metricRsl := "success"

	defer func() {
		s.metrics.TripPublishTotal.WithLabelValues(metricRsl).Inc()
		s.metrics.TripPublishDuration.WithLabelValues(metricRsl).
			Observe(time.Since(started).Seconds())
	}()

	result, err := tx(
		ctx,
		s.pool,
		func(tx pgx.Tx) (*uuid.UUID, error) {
			return s.tripUseCase.MoveTripDraftToPublish(ctx, tx, tripDomain)
		},
	)

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSkipOperation):
			metricRsl = "already_published"
			return tripDomain.TripID, err
		case errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrForbidden):
			metricRsl = "conflict"
		default:
			metricRsl = "internal_error"
		}

		return uuid.Nil, err
	}

	if result == nil {
		return uuid.Nil, errors.New("publish trip returned nil UUID")
	}

	return *result, nil
}
