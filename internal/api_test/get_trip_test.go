package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/stretchr/testify/require"
)

type getTripTestResponse struct {
	Data   *dto.GetTripResponse `json:"data"`
	Errors []string             `json:"errors"`
}

func sendGetTripRequest(
	t *testing.T,
	tripID string,
) getTripTestResponse {
	t.Helper()

	return sendJSONRequest[getTripTestResponse](
		t,
		http.MethodGet,
		tripTestURL+tripID,
		nil,
		http.StatusOK,
	)
}

func TestServer_GetTrip(t *testing.T) {
	t.Parallel()

	t.Run("получение существующей поездки", func(t *testing.T) {
		t.Parallel()
		clientID := uuid.New()
		departureTime := time.Now().
			UTC().
			Add(time.Hour).
			Truncate(time.Second)

		created := sendCreateTripRequest(t, dto.CreateTripRequest{
			ClientID:       clientID.String(),
			FromPoint:      "Москва",
			ToPoint:        "Коломна",
			DepartureTime:  departureTime.Format(time.RFC3339),
			AvailableSeats: 2,
		})

		require.Empty(t, created.Errors)
		require.NotNil(t, created.Data)

		response := sendGetTripRequest(t, created.Data.ID.String())

		require.Empty(t, response.Errors)
		require.NotNil(t, response.Data)

		got := response.Data

		require.Equal(t, created.Data.ID, got.ID)
		require.Equal(t, clientID, got.ClientID)
		require.Equal(t, "Москва", got.FromPoint)
		require.Equal(t, "Коломна", got.ToPoint)
		require.True(
			t,
			departureTime.Equal(got.DepartureTime),
			"expected %s, got %s",
			departureTime,
			got.DepartureTime,
		)
		require.Equal(t, 2, got.AvailableSeats)
		require.Equal(t, string(domain.TripStatusDraft), got.Status)
		require.False(t, got.CreatedAt.IsZero())

		// PostgreSQL сохраняет timestamp с микросекундной точностью.
		require.WithinDuration(
			t,
			created.Data.CreatedAt,
			got.CreatedAt,
			time.Microsecond,
		)
	})

	t.Run("несуществующая поездка", func(t *testing.T) {
		t.Parallel()
		response := sendGetTripRequest(t, uuid.NewString())

		require.Nil(t, response.Data)
		require.NotEmpty(t, response.Errors)
		require.Contains(
			t,
			strings.Join(response.Errors, "; "),
			domain.ErrNotFound.Error(),
		)
	})
}

func TestServer_GetTrip_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		id   string
	}{
		{
			name: "произвольная строка",
			id:   "not-a-uuid",
		},
		{
			name: "слишком короткий UUID",
			id:   "550e8400-e29b-41d4-a716",
		},
		{
			name: "недопустимые символы",
			id:   "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			response := sendGetTripRequest(t, tt.id)

			require.Nil(t, response.Data)
			require.Equal(
				t,
				[]string{"invalid trip id"},
				response.Errors,
			)
		})
	}
}
