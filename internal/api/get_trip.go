package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func (s *Server) GetTrip(c *fiber.Ctx) error {
	tracer := otel.Tracer("trip-api")

	ctx, span := tracer.Start(c.UserContext(), "GetTripHandler")
	defer span.End()

	c.Set("trace-id", span.SpanContext().TraceID().String())

	tripID, err := uuid.Parse(c.Params("tripId"))
	if err != nil {
		return Failure(c, "invalid trip id")
	}

	span.SetAttributes(
		attribute.String("trip_id", tripID.String()),
	)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	trip, err := s.tripService.GetTripById(ctx, tripID)
	if err != nil {
		return HandleError(c, err)
	}
	return Success(c, dto.GetTripResponseFromTripDomain(trip))
}
