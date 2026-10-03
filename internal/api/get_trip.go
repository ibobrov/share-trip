package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api/dto"
)

func (s *Server) GetTrip(c *fiber.Ctx) error {
	tripID, err := uuid.Parse(c.Params("tripId"))
	if err != nil {
		return Failure(c, "invalid trip id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	trip, err := s.tripService.GetTripById(ctx, tripID)
	if err != nil {
		return HandleError(c, err)
	}
	return Success(c, dto.GetTripResponseFromTripDomain(trip))
}
