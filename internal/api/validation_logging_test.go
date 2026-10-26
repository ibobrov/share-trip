package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestValidationLogDoesNotContainSubmittedValue(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		var request struct {
			ID string `json:"id" valid:"required,uuid"`
		}
		_, err := ParseAndValidateRequest(&request, c, logger)
		return err
	})
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":"PRIVATE_TEST_VALUE"}`))
	request.Header.Set("Content-Type", "application/json")

	response, err := app.Test(request)

	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, logs.String(), `"level":"WARN"`)
	require.Contains(t, logs.String(), `"fields":["id"]`)
	require.NotContains(t, logs.String(), "PRIVATE_TEST_VALUE")
}
