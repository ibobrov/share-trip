package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
)

func (s *Server) PublishTrip(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger := logctx.Logger(ctx).With(
		slog.String("server", "TripServer"),
		slog.String("handler", "PublishTrip"),
	)

	var request dto.PublishTripRequest
	ok, err := ParseAndValidateRequest(&request, c, logger)
	if !ok {
		return err
	}

	logger = logger.With(
		slog.String("client_id", request.ClientID),
	)

	ctx, cancel := context.WithTimeout(logctx.WithLogger(ctx, logger), 2*time.Second)
	defer cancel()

	logger.Info("publish trip request accepted")

	publishedTripId, err := s.tripService.PublishTrip(ctx, request.ToPublishTripDomain())
	if err != nil {
		logger.Error(
			"publish trip failed",
			slog.Any("error", err),
		)
		return HandleError(c, err)
	}

	logger.Info(
		"publish trip completed",
		slog.String("trip_id", publishedTripId.String()),
	)

	return Success(c, dto.PublishTripResponse{TripID: publishedTripId})
}
