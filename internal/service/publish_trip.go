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
	"go.opentelemetry.io/otel"
)

type PublishTripRequest struct {
	TripID   uuid.UUID
	ClientID uuid.UUID
}

type PublishTripResponse struct {
	TripID             uuid.UUID
	IsAlreadyPublished bool
}

func (s *TripService) PublishTrip(
	ctx context.Context,
	req PublishTripRequest,
) (PublishTripResponse, error) {
	ctx, span := otel.Tracer("TripService").Start(ctx, "TripService.PublishTrip")
	defer span.End()

	started := time.Now()
	metricRsl := "success"

	defer func() {
		s.metrics.TripPublishTotal.WithLabelValues(metricRsl).Inc()
		s.metrics.TripPublishDuration.WithLabelValues(metricRsl).
			Observe(time.Since(started).Seconds())
	}()

	logger := logctx.Logger(ctx).With(
		slog.String("service", "TripService"),
		slog.String("operation", "PublishTrip"),
		slog.String("layer", "service"),
	)

	result, err := tx(
		ctx,
		s.pool,
		func(tx pgx.Tx) (*PublishTripResponse, error) {
			res, err := s.tripUseCase.MoveTripDraftToPublish(ctx, tx, domain.PublishTripRequest{
				TripID:   req.TripID,
				ClientID: req.ClientID,
			})

			if err != nil {
				logger.Error(
					"publish trip usecase failed",
					slog.String("layer", "transaction"),
					slog.Any("error", err),
				)
				return nil, fmt.Errorf("usecase.MoveTripDraftToPublish: %w", err)
			}

			if !res.IsAlreadyPublished {
				err = s.tripUseCase.SentNotificationTripPublish(ctx, tx, domain.SentNotificationTripPublishRequest{
					TripID: req.TripID,
				})

				if err != nil {
					logger.Error(
						"sent notification trip publish usecase failed",
						slog.String("layer", "transaction"),
						slog.Any("error", err),
					)
					return nil, fmt.Errorf("usecase.SentNotificationTripPublish: %w", err)
				}
			}

			return &PublishTripResponse{
				TripID:             res.TripID,
				IsAlreadyPublished: res.IsAlreadyPublished,
			}, nil
		},
	)

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrForbidden):
			metricRsl = "conflict"
		default:
			metricRsl = "internal_error"
		}

		return PublishTripResponse{}, fmt.Errorf("tripService.PublishTrip:%w", err)
	}

	return *result, nil
}
