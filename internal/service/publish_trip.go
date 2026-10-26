package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type PublishTripRequest struct {
	TripID   uuid.UUID
	ClientID uuid.UUID
}

type PublishTripResponse struct {
	TripID             uuid.UUID
	EventID            uuid.UUID
	IsAlreadyPublished bool
}

func (s *TripService) PublishTrip(
	ctx context.Context,
	req PublishTripRequest,
) (response PublishTripResponse, err error) {
	ctx, span := otel.Tracer("sharetrip").Start(ctx, "TripService.PublishTrip")
	defer span.End()
	ctx = logctx.WithBusinessIDs(ctx, req.TripID.String(), req.ClientID.String())

	started := time.Now()

	defer func() {
		result := metricResult(err)
		if result == "internal_error" {
			span.RecordError(err)
			span.SetStatus(codes.Error, result)
		}
		logctx.Logger(ctx).Debug("trip publication completed", "component", "TripService", "operation", "PublishTrip", "result", result)
		s.metrics.TripPublishTotal.WithLabelValues(result).Inc()
		s.metrics.TripPublishDuration.WithLabelValues(result).
			Observe(time.Since(started).Seconds())
	}()

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
				return nil, fmt.Errorf("usecase.MoveTripDraftToPublish: %w", err)
			}

			event, err := s.tripUseCase.SentNotificationTripPublish(ctx, tx, domain.SentNotificationTripPublishRequest{
				TripID:        res.TripID,
				ClientID:      res.ClientID,
				CompanyID:     res.CompanyID,
				OccurredAt:    res.PublishedAt,
				RequestID:     logctx.RequestID(ctx),
				CorrelationID: logctx.CorrelationID(ctx),
				CausationID:   logctx.RequestID(ctx),
				Traceparent:   logctx.Traceparent(ctx),
				Tracestate:    logctx.Tracestate(ctx),
			})

			if err != nil {
				return nil, fmt.Errorf("usecase.SentNotificationTripPublish: %w", err)
			}

			return &PublishTripResponse{TripID: res.TripID, EventID: event.EventID}, nil
		},
	)

	if err != nil {
		return PublishTripResponse{}, fmt.Errorf("tripService.PublishTrip:%w", err)
	}

	return *result, nil
}
