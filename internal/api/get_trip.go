package api

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/observability/middleware"
	"github.com/ibobrov/share_trip/internal/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type GetTripResponse struct {
	ID             uuid.UUID `json:"id"`
	ClientID       uuid.UUID `json:"client_id"`
	CompanyID      string    `json:"company_id"`
	FromPoint      string    `json:"from_point"`
	ToPoint        string    `json:"to_point"`
	DepartureTime  time.Time `json:"departure_time"`
	AvailableSeats int       `json:"available_seats"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Server) GetTrip(c *fiber.Ctx) error {
	tracer := otel.Tracer("trip-api")

	ctx, span := tracer.Start(c.UserContext(), "GetTripHandler")
	defer span.End()

	c.Set("trace-id", span.SpanContext().TraceID().String())

	claims, err := middleware.ClaimsFromContext(c)
	if err != nil {
		return err
	}
	sub := claims.Subject

	logger := logctx.Logger(ctx).With(
		slog.String("server", "TripServer"),
		slog.String("handler", "GetTrip"),
		slog.String("client_id", sub),
	)
	logger.Info("get trip request accepted")

	tripID, err := uuid.Parse(c.Params("tripId"))
	if err != nil {
		return Failure(c, "invalid trip id")
	}

	span.SetAttributes(
		attribute.String("trip_id", tripID.String()),
	)

	res, err := s.tripService.GetTripByIdRequest(ctx, service.GetTripRequest{TripID: tripID})
	if err != nil {
		return HandleError(c, func() {
			logger.Error(
				"get trip failed",
				slog.Any("error", err),
			)
		}, err)
	}
	return Success(c, GetTripResponse{
		ID:             res.ID,
		ClientID:       res.ClientID,
		CompanyID:      res.CompanyID,
		FromPoint:      res.FromPoint,
		ToPoint:        res.ToPoint,
		DepartureTime:  res.DepartureTime,
		AvailableSeats: res.AvailableSeats,
		Status:         res.Status,
		CreatedAt:      res.CreatedAt,
	})
}
