package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api/dto"
)

func (s *Server) PublishTrip(c *fiber.Ctx) error {
	var request dto.PublishTripRequest
	ok, err := ParseAndValidateRequest(&request, c)
	if !ok {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	publishedTripId, err := s.tripService.PublishTrip(ctx, request.ToPublishTripDomain())
	if err != nil {
		return HandleError(c, err)
	}
	return Success(c, dto.PublishTripResponse{TripID: publishedTripId})
}
