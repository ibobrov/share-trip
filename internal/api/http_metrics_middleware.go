package api

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/ibobrov/share_trip/internal/observability/metrics"
)

func NewHTTPMetricsMiddleware(m *metrics.Metrics) fiber.Handler {
	return func(c *fiber.Ctx) error {
		started := time.Now()

		err := c.Next()

		status := strconv.Itoa(c.Response().StatusCode())
		path := c.Route().Path

		if path == "" {
			path = c.Path()
		}

		m.HTTPRequestTotal.WithLabelValues(
			c.Method(),
			path,
			status,
		).Inc()

		m.HTTPRequestDuration.WithLabelValues(
			c.Method(),
			path,
			status,
		).Observe(time.Since(started).Seconds())

		return err
	}
}
