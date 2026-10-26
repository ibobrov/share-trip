package api

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/domain"
)

func HandleError(c *fiber.Ctx, logger *slog.Logger, err error) error {
	switch {
	case err == nil:
		return nil

	case errors.Is(err, domain.ErrNotFound):
		logger.Info("request rejected", "result", "not_found")
		return FailureWithHttpStatus(c, fiber.StatusNotFound, err.Error())

	case errors.Is(err, domain.ErrForbidden):
		logger.Info("request rejected", "result", "forbidden")
		return FailureWithHttpStatus(c, fiber.StatusForbidden, err.Error())

	case errors.Is(err, domain.ErrConflict):
		logger.Info("request rejected", "result", "conflict")
		return FailureWithHttpStatus(c, fiber.StatusConflict, err.Error())

	case errors.Is(err, domain.ErrDomain):
		logger.Info("request rejected", "result", "validation_error")
		return Failure(c, err.Error())

	default:
		logger.Error("request failed", "result", "internal_error", "error", err)
		return FailureWithHttpStatus(c, fiber.StatusInternalServerError, err.Error())
	}
}
