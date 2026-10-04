package api

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/domain"
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
		if errors.Is(err, domain.ErrSkipOperation) {
			return SuccessWithHttpStatus(c, fiber.StatusOK, dto.PublishTripResponse{TripID: publishedTripId})
		}

		return HandleError(c, err)
	}
	return SuccessWithHttpStatus(c, fiber.StatusCreated, dto.PublishTripResponse{TripID: publishedTripId})
}
