package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetricsMiddleware_Status(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    fiber.Handler
		wantStatus int
	}{
		{
			name:       "successful response",
			handler:    func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "explicit error response",
			handler:    func(c *fiber.Ctx) error { return c.SendStatus(http.StatusForbidden) },
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "returned Fiber error is not counted as HTTP 200",
			handler:    func(*fiber.Ctx) error { return fiber.ErrNotFound },
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "returned internal error is not counted as HTTP 200",
			handler:    func(*fiber.Ctx) error { return errors.New("handler failed") },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			httpMetrics := metrics.New(prometheus.NewRegistry())
			app := fiber.New()
			app.Use(api.NewHTTPMetricsMiddleware(httpMetrics))
			app.Get("/example", tt.handler)

			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/example", nil))
			require.NoError(t, err)
			defer func() { require.NoError(t, response.Body.Close()) }()
			require.Equal(t, tt.wantStatus, response.StatusCode)

			var counter dto.Metric
			err = httpMetrics.HTTPRequestTotal.WithLabelValues("GET", "/example", strconv.Itoa(tt.wantStatus)).Write(&counter)
			require.NoError(t, err)
			require.InDelta(t, 1, counter.GetCounter().GetValue(), 0.001)
		})
	}
}
