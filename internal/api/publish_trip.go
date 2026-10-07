package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/domain"
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
		if errors.Is(err, domain.ErrSkipOperation) {
			return SuccessWithHttpStatus(c, fiber.StatusOK, dto.PublishTripResponse{TripID: publishedTripId})
		}

		return HandleError(c, err)
	}

	logger.Info(
		"publish trip completed",
		slog.String("trip_id", publishedTripId.String()),
	)

	return SuccessWithHttpStatus(c, fiber.StatusCreated, dto.PublishTripResponse{TripID: publishedTripId})
}
