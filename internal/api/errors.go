package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/domain"
)

func HandleError(c *fiber.Ctx, err error) error {
	switch {
	case err == nil:
		return nil

	case errors.Is(err, domain.ErrNotFound):
		return FailureWithHttpStatus(c, fiber.StatusNotFound, err.Error())

	case errors.Is(err, domain.ErrForbidden):
		return FailureWithHttpStatus(c, fiber.StatusForbidden, err.Error())

	case errors.Is(err, domain.ErrConflict):
		return FailureWithHttpStatus(c, fiber.StatusConflict, err.Error())

	case errors.Is(err, domain.ErrDomain):
		return Failure(c, err.Error())

	default:
		return FailureWithHttpStatus(c, fiber.StatusInternalServerError, err.Error())
	}
}
