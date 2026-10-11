package api

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/service"
)

type CreateTripRequest struct {
	ClientID       string `json:"client_id" valid:"required,uuid"`
	FromPoint      string `json:"from_point" valid:"required,matches(\\S)"`
	ToPoint        string `json:"to_point" valid:"required,matches(\\S)"`
	DepartureTime  string `json:"departure_time" valid:"required,rfc3339"`
	AvailableSeats int    `json:"available_seats" valid:"required"`
}

type CreateTripResponse struct {
	ID             uuid.UUID `json:"id"`
	ClientID       uuid.UUID `json:"client_id"`
	FromPoint      string    `json:"from_point"`
	ToPoint        string    `json:"to_point"`
	DepartureTime  time.Time `json:"departure_time"`
	AvailableSeats int       `json:"available_seats"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Server) CreateTrip(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger := logctx.Logger(ctx).With(
		slog.String("server", "TripServer"),
		slog.String("handler", "CreateTrip"),
	)

	var req CreateTripRequest
	ok, err := ParseAndValidateRequest(&req, c, logger)
	if !ok {
		return err
	}

	logger = logger.With(
		slog.String("client_id", req.ClientID),
	)
	logger.Info("create trip request accepted")

	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		return fmt.Errorf("when parse clientID: %w", err)
	}

	departureTime, err := time.Parse(time.RFC3339, req.DepartureTime)
	if err != nil {
		return fmt.Errorf("when parse departureTime: %w", err)
	}

	res, err := s.tripService.CreateTrip(ctx, service.CreateTripRequest{
		ClientID:       clientID,
		FromPoint:      req.FromPoint,
		ToPoint:        req.ToPoint,
		DepartureTime:  departureTime,
		AvailableSeats: req.AvailableSeats,
	})
	if err != nil {
		return HandleError(c, func() {
			logger.Error(
				"create trip failed",
				slog.Any("error", err),
			)
		}, err)
	}

	return Success(c, CreateTripResponse{
		ID:             res.ID,
		ClientID:       res.ClientID,
		FromPoint:      res.FromPoint,
		ToPoint:        res.ToPoint,
		DepartureTime:  res.DepartureTime,
		AvailableSeats: res.AvailableSeats,
		Status:         res.Status,
		CreatedAt:      res.CreatedAt,
	})
}
