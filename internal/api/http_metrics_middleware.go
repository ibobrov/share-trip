package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/ibobrov/share_trip/internal/observability/metrics"
)

func NewHTTPMetricsMiddleware(m *metrics.Metrics) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		started := time.Now()

		defer func() {
			statusCode := c.Response().StatusCode()
			if err != nil {
				statusCode = http.StatusInternalServerError
				var httpError *fiber.Error
				if errors.As(err, &httpError) {
					statusCode = httpError.Code
				}
			}
			status := strconv.Itoa(statusCode)
			path := c.Route().Path
			if path == "" {
				path = c.Path()
			}

			m.HTTPRequestTotal.WithLabelValues(c.Method(), path, status).Inc()
			m.HTTPRequestDuration.WithLabelValues(c.Method(), path, status).
				Observe(time.Since(started).Seconds())
		}()

		return c.Next()
	}
}
