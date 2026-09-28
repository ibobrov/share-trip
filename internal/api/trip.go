package api

import (
	"context"
	"errors"
	"time"

	"github.com/asaskevich/govalidator/v12"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (server *Server) CreateTrip(c *fiber.Ctx) error {
	var request dto.CreateTripRequest

	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if _, err := govalidator.ValidateStruct(request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "validation_failed",
			"details": govalidator.ErrorsByField(err),
		})
	}

	tripDomain := request.ToNewTripDomain()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	createdTrip, err := server.tripService.CreateTrip(ctx, tripDomain)
	if err != nil {
		switch {
		case errors.Is(err, domain.IncorrectTripDepartureTime):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})

		case errors.Is(err, domain.IncorrectTripSeats):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})

		case errors.Is(err, context.DeadlineExceeded),
			errors.Is(ctx.Err(), context.DeadlineExceeded):
			return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{
				"error": "request timed out",
			})

		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to create dto",
			})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(dto.CreateTripResponseFromTripDomain(createdTrip))
}

func (server *Server) GetTrip(c *fiber.Ctx) error {
	tripID, err := uuid.Parse(c.Params("tripId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid dto id",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	trip, err := server.tripService.GetById(ctx, tripID)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded),
			errors.Is(ctx.Err(), context.DeadlineExceeded):
			return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{
				"error": "request timed out",
			})

		case errors.Is(err, pgx.ErrNoRows):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "dto not found",
			})

		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to get dto",
			})
		}
	}

	return c.Status(fiber.StatusOK).JSON(trip)
}
