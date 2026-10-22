package service

import (
	"errors"

	"github.com/ibobrov/share_trip/internal/domain"
)

func metricResult(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, domain.ErrForbidden):
		return "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrConflict):
		return "conflict"
	case errors.Is(err, domain.ErrTripIsAlreadyPublished):
		return "already_published"
	case errors.Is(err, domain.ErrDomain):
		return "validation_error"
	default:
		return "internal_error"
	}
}
