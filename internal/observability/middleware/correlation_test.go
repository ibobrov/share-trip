package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/stretchr/testify/require"
)

func TestCorrelationPreservesIDsAfterRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(Correlation(slog.New(slog.NewTextHandler(io.Discard, nil))))
	var contexts []context.Context
	app.Get("/", func(c *fiber.Ctx) error {
		contexts = append(contexts, c.UserContext())
		return c.SendStatus(200)
	})
	for _, id := range []string{"first-request", "other-request"} {
		request := httptest.NewRequest("GET", "/", nil)
		request.Header.Set(RequestIDHeader, id)
		response, err := app.Test(request)
		require.NoError(t, err)
		require.Equal(t, id, response.Header.Get(CorrelationIDHeader))
		require.NoError(t, response.Body.Close())
	}
	require.Equal(t, "first-request", logctx.RequestID(contexts[0]))
	require.Equal(t, "first-request", logctx.CorrelationID(contexts[0]))
	require.Equal(t, "other-request", logctx.RequestID(contexts[1]))
}
