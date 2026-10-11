package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type PublishTripRequest struct {
	TripID   string `json:"trip_id" valid:"required,uuid"`
	ClientID string `json:"client_id" valid:"required,uuid"`
}

type PublishTripResponse struct {
	TripID uuid.UUID `json:"trip_id"`
}

func (s *Server) PublishTrip(c *fiber.Ctx) error {
	tracer := otel.Tracer("trip-api")

	ctx, span := tracer.Start(c.UserContext(), "PublishTripHandler")
	defer span.End()

	c.Set("trace-id", span.SpanContext().TraceID().String())

	logger := logctx.Logger(ctx).With(
		slog.String("server", "TripServer"),
		slog.String("handler", "PublishTrip"),
	)

	var req PublishTripRequest
	ok, err := ParseAndValidateRequest(&req, c, logger)
	if !ok {
		return err
	}

	span.SetAttributes(
		attribute.String("trip_id", req.TripID),
		attribute.String("client_id", req.ClientID),
	)

	logger = logger.With(
		slog.String("client_id", req.ClientID),
	)

	logger.Info("publish trip request accepted")

	tripID, _ := uuid.Parse(req.TripID)
	clientID, _ := uuid.Parse(req.ClientID)

	res, err := s.tripService.PublishTrip(ctx, service.PublishTripRequest{
		TripID:   tripID,
		ClientID: clientID,
	})

	if err != nil {
		logger.Error(
			"publish trip failed",
			slog.Any("error", err),
		)

		return HandleError(c, err)
	}

	logger.Info(
		"publish trip completed",
		slog.String("trip_id", req.TripID),
	)

	if res.IsAlreadyPublished {
		return Success(c, PublishTripResponse{TripID: res.TripID})
	}

	return SuccessWithHttpStatus(c, fiber.StatusCreated, PublishTripResponse{TripID: res.TripID})
}
