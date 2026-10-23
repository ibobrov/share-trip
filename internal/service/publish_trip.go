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
) (response PublishTripResponse, err error) {
	ctx, span := otel.Tracer("TripService").Start(ctx, "TripService.PublishTrip")
	defer span.End()

	started := time.Now()

	defer func() {
		result := metricResult(err)
		s.metrics.TripPublishTotal.WithLabelValues(result).Inc()
		s.metrics.TripPublishDuration.WithLabelValues(result).
			Observe(time.Since(started).Seconds())
	}()

	logger := logctx.Logger(ctx).With(
		slog.String("service", "TripService"),
		slog.String("operation", "PublishTrip"),
		slog.String("layer", "service"),
	)

	trip, err := s.tripUseCase.GetTripForPublication(ctx, domain.GetTripForPublicationRequest{TripID: req.TripID})
	if err != nil {
		return PublishTripResponse{}, fmt.Errorf("s.tripUseCase.GetTripForPublication: %w", err)
	}
	if trip.ClientID != req.ClientID {
		return PublishTripResponse{}, fmt.Errorf("%w: client %s is not driver of trip %s", domain.ErrForbidden, req.ClientID, req.TripID)
	}
	if trip.Status == string(domain.TripStatusPublished) {
		return PublishTripResponse{}, domain.ErrTripIsAlreadyPublished
	}

	option, err := s.contractClient.CheckOption(ctx, trip.CompanyID, "trip_publication")
	if err != nil {
		return PublishTripResponse{}, fmt.Errorf("cs.contractClient.CheckOption: %w", err)
	}
	if !option.Allowed {
		return PublishTripResponse{}, fmt.Errorf("!option.Allowed: %w: trip_publication is unavailable: %s", domain.ErrForbidden, option.Reason)
	}

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

			_, err = s.tripUseCase.SentNotificationTripPublish(ctx, tx, domain.SentNotificationTripPublishRequest{
				TripID:     res.TripID,
				ClientID:   res.ClientID,
				CompanyID:  res.CompanyID,
				OccurredAt: res.PublishedAt,
			})

			if err != nil {
				logger.Error(
					"sent notification trip publish usecase failed",
					slog.String("layer", "transaction"),
					slog.Any("error", err),
				)
				return nil, fmt.Errorf("usecase.SentNotificationTripPublish: %w", err)
			}

			return &PublishTripResponse{TripID: res.TripID}, nil
		},
	)

	if err != nil {
		return PublishTripResponse{}, fmt.Errorf("tripService.PublishTrip:%w", err)
	}

	return *result, nil
}
