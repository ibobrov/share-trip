package api

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/observability/middleware"
	"github.com/ibobrov/share_trip/internal/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type PublishTripRequest struct {
	TripID string `json:"trip_id" valid:"required,uuid"`
}

type PublishTripResponse struct {
	TripID uuid.UUID `json:"trip_id"`
}

func (s *Server) PublishTrip(c *fiber.Ctx) error {
	tracer := otel.Tracer("sharetrip")

	ctx, span := tracer.Start(c.UserContext(), "PublishTripHandler")
	defer span.End()

	c.Set("trace-id", span.SpanContext().TraceID().String())

	claims, err := middleware.ClaimsFromContext(c)
	if err != nil {
		return err
	}
	sub := claims.Subject

	ctx = logctx.WithBusinessIDs(ctx, "", sub)
	logger := logctx.Logger(ctx).With(
		slog.String("component", "TripServer"),
		slog.String("operation", "PublishTrip"),
	)

	var req PublishTripRequest
	ok, err := ParseAndValidateRequest(&req, c, logger)
	if !ok {
		return err
	}

	span.SetAttributes(
		attribute.String("trip_id", req.TripID),
		attribute.String("client_id", sub),
	)

	logger.Debug("publish trip request accepted")

	tripID, err := uuid.Parse(req.TripID)
	if err != nil {
		return fmt.Errorf("when parse tripID: %w", err)
	}

	clientID, err := uuid.Parse(sub)
	if err != nil {
		return fmt.Errorf("when parse clientID: %w", err)
	}

	res, err := s.tripService.PublishTrip(ctx, service.PublishTripRequest{
		TripID:   tripID,
		ClientID: clientID,
	})

	if err != nil {
		if errors.Is(err, domain.ErrTripIsAlreadyPublished) {
			logger.Info("request completed", "result", "already_published", "trip_id", tripID)
			return Success(c, PublishTripResponse{TripID: tripID})
		}

		return HandleError(c, logger, err)
	}

	logger.Info("request completed", "result", "success", "trip_id", res.TripID, "event_id", res.EventID)
	return SuccessWithHttpStatus(c, fiber.StatusCreated, PublishTripResponse{TripID: res.TripID})
}
