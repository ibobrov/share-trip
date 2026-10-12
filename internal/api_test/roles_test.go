package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api"
	"github.com/stretchr/testify/require"
)

func TestServer_TripRoutes_AccessDenied(t *testing.T) {
	t.Parallel()

	routes := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{
			name:   "получение",
			method: http.MethodGet,
			url:    tripTestURL + uuid.NewString(),
		},
		{
			name:   "создание",
			method: http.MethodPost,
			url:    tripTestURL + "create",
			body:   `{}`,
		},
		{
			name:   "публикация",
			method: http.MethodPost,
			url:    tripTestURL + "publish",
			body:   `{}`,
		},
	}

	cases := []struct {
		name   string
		userID string
		role   string
		status int
	}{
		{
			name:   "без аутентификации",
			status: fiber.StatusUnauthorized,
		},
		{
			name:   "пользователь без роли",
			userID: uuid.NewString(),
			status: fiber.StatusForbidden,
		},
		{
			name:   "пользователь с другой ролью",
			userID: uuid.NewString(),
			role:   "driver",
			status: fiber.StatusForbidden,
		},
	}

	for _, route := range routes {
		for _, tc := range cases {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequest(
					route.method,
					route.url,
					strings.NewReader(route.body),
				)
				require.NoError(t, err)
				req.Header.Set("Accept", "application/json")

				if route.body != "" {
					req.Header.Set("Content-Type", "application/json")
				}
				if tc.userID != "" {
					req.Header.Set(testUserHeader, tc.userID)
				}
				if tc.role != "" {
					req.Header.Set(testRoleHeader, tc.role)
				}

				resp, err := testApp.Test(req, -1)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, tc.status, resp.StatusCode)
			})
		}
	}
}

func TestServer_TripRoutes_ClientRoleAllowed(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()

	// Пользователь с ролью client может создать поездку.
	created := sendCreateTripRequest(
		t,
		fiber.StatusOK,
		api.CreateTripRequest{
			FromPoint: "Москва",
			ToPoint:   "Коломна",
			DepartureTime: time.Now().
				UTC().
				Add(time.Hour).
				Truncate(time.Second).
				Format(time.RFC3339),
			AvailableSeats: 2,
		},
		withUser(clientID),
	)

	require.Empty(t, created.Errors)
	require.NotNil(t, created.Data)
	require.Equal(t, clientID, created.Data.ClientID)

	// Тот же пользователь может получить поездку.
	got := sendGetTripRequest(
		t,
		fiber.StatusOK,
		created.Data.ID.String(),
		withUser(clientID),
	)

	require.Empty(t, got.Errors)
	require.NotNil(t, got.Data)
	require.Equal(t, created.Data.ID, got.Data.ID)
	require.Equal(t, clientID, got.Data.ClientID)

	// И опубликовать свою поездку.
	published := sendPublishTripRequest(
		t,
		fiber.StatusCreated,
		api.PublishTripRequest{
			TripID: created.Data.ID.String(),
		},
		withUser(clientID),
	)

	require.Empty(t, published.Errors)
	require.NotNil(t, published.Data)
	require.Equal(t, created.Data.ID, published.Data.TripID)
}
