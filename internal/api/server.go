package api

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/asaskevich/govalidator/v12"
	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB          *pgxpool.Pool
	tripService *service.TripService
}

func NewServer(db *pgxpool.Pool, tripService *service.TripService) *Server {
	return &Server{
		DB:          db,
		tripService: tripService,
	}
}

func ParseAndValidateRequest[T interface{}](request *T, c *fiber.Ctx, logger *slog.Logger) (ok bool, err error) {
	if err := c.BodyParser(request); err != nil {
		logger.Warn(
			"create trip failed: invalid json body",
			slog.Any("error", err),
			slog.String("layer", "validation"),
		)
		return false, Failure(c, "некорректное тело запроса")
	}

	if _, err := govalidator.ValidateStruct(*request); err != nil {
		validationErrors := make([]string, 0)

		for field, message := range govalidator.ErrorsByField(err) {
			validationErrors = append(validationErrors,
				fmt.Sprintf("field of request `%s` contains error `%s`", field, message),
			)
		}

		// На случай ошибки, которую нельзя привязать к полю.
		if len(validationErrors) == 0 {
			validationErrors = append(validationErrors, err.Error())
		}

		logger.Warn(
			"create trip failed:",
			slog.Any("error", validationErrors),
			slog.String("layer", "validation"),
		)

		return false, Failure(c, validationErrors...)
	}

	return true, nil
}

func HandleError(c *fiber.Ctx, err error) error {
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			return FailureWithHttpStatus(c, fiber.StatusNotFound, err.Error())

		case errors.Is(err, domain.ErrForbidden):
			return FailureWithHttpStatus(c, fiber.StatusForbidden, err.Error())

		case errors.Is(err, domain.ErrConflict):
			return FailureWithHttpStatus(c, fiber.StatusConflict, err.Error())

		case errors.Is(err, domain.ErrSkipOperation):
			return FailureWithHttpStatus(c, fiber.StatusNoContent, err.Error())

		case errors.Is(err, domain.ErrDomain):
			return Failure(c, err.Error())

		default:
			return FailureWithHttpStatus(c, fiber.StatusInternalServerError, err.Error())
		}
	}

	return nil
}
