package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
)

func (s *Server) CreateTrip(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger := logctx.Logger(ctx).With(
		slog.String("server", "TripServer"),
		slog.String("handler", "CreateTrip"),
	)

	var request dto.CreateTripRequest
	ok, err := ParseAndValidateRequest(&request, c, logger)
	if !ok {
		return err
	}

	logger = logger.With(
		slog.String("client_id", request.ClientID),
	)

	ctx, cancel := context.WithTimeout(logctx.WithLogger(ctx, logger), 2*time.Second)
	defer cancel()

	logger.Info("create trip request accepted")

	createdTrip, err := s.tripService.CreateTrip(ctx, request.ToNewTripDomain())
	if err != nil {
		logger.Error(
			"create trip failed",
			slog.Any("error", err),
		)
		return HandleError(c, err)
	}

	logger.Info(
		"create trip completed",
		slog.String("trip_id", createdTrip.ID.String()),
	)

	return Success(c, dto.CreateTripResponseFromTripDomain(createdTrip))
}
