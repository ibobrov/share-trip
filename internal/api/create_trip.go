package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api/dto"
)

func (s *Server) CreateTrip(c *fiber.Ctx) error {
	var request dto.CreateTripRequest
	ok, err := ParseAndValidateRequest(&request, c)
	if !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	createdTrip, err := s.tripService.CreateTrip(ctx, request.ToNewTripDomain())
	if err != nil {
		return HandleError(c, err)
	}
	return Success(c, dto.CreateTripResponseFromTripDomain(createdTrip))
}
