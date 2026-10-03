package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/stretchr/testify/require"
)

type createTripTestResponse struct {
	Data   *dto.CreateTripResponse `json:"data"`
	Errors []string                `json:"errors"`
}

func sendCreateTripRequest(
	t *testing.T,
	payload dto.CreateTripRequest,
) createTripTestResponse {
	t.Helper()

	return sendJSONRequest[createTripTestResponse](
		t,
		http.MethodPost,
		tripTestURL+"create",
		payload,
		http.StatusOK,
	)
}

func TestServer_CreateTrip(t *testing.T) {
	t.Parallel()
	t.Run("успешное создание поездки, истории", func(t *testing.T) {
		clientID := uuid.New()
		fromPoint := "Москва"
		toPoint := "Владивосток"
		departureTime := time.Now().
			UTC().
			Add(time.Hour).
			Truncate(time.Second)
		availableSeats := 1

		response := sendCreateTripRequest(t, dto.CreateTripRequest{
			ClientID:       clientID.String(),
			FromPoint:      fromPoint,
			ToPoint:        toPoint,
			DepartureTime:  departureTime.Format(time.RFC3339),
			AvailableSeats: availableSeats,
		})

		require.Empty(t, response.Errors)
		require.NotNil(t, response.Data)

		got := response.Data

		require.NotEqual(t, uuid.Nil, got.ID)
		require.Equal(t, clientID, got.ClientID)
		require.Equal(t, fromPoint, got.FromPoint)
		require.Equal(t, toPoint, got.ToPoint)
		require.True(
			t,
			departureTime.Equal(got.DepartureTime),
			"expected departure time %s, got %s",
			departureTime,
			got.DepartureTime,
		)
		require.Equal(t, availableSeats, got.AvailableSeats)
		require.Equal(t, string(domain.TripStatusDraft), string(got.Status))
		require.False(t, got.CreatedAt.IsZero())

		ctx, cancel := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		defer cancel()

		// Проверяем фактическое сохранение поездки.
		var (
			savedClientID      uuid.UUID
			savedFromPoint     string
			savedToPoint       string
			savedDepartureTime time.Time
			savedSeats         int
			savedStatus        string
		)

		err := testPool.QueryRow(
			ctx,
			`
				SELECT
					client_id,
					from_point,
					to_point,
					departure_time,
					seats,
					status
				FROM trips
				WHERE id = $1
			`,
			got.ID,
		).Scan(
			&savedClientID,
			&savedFromPoint,
			&savedToPoint,
			&savedDepartureTime,
			&savedSeats,
			&savedStatus,
		)
		require.NoError(t, err)

		require.Equal(t, clientID, savedClientID)
		require.Equal(t, fromPoint, savedFromPoint)
		require.Equal(t, toPoint, savedToPoint)
		require.True(t, departureTime.Equal(savedDepartureTime))
		require.Equal(t, availableSeats, savedSeats)
		require.Equal(t, string(domain.TripStatusDraft), savedStatus)

		// При создании должна появиться ровно одна запись истории.
		var historyCount int

		err = testPool.QueryRow(
			ctx,
			`SELECT count(*) FROM trip_history WHERE trip_id = $1`,
			got.ID,
		).Scan(&historyCount)
		require.NoError(t, err)
		require.Equal(t, 1, historyCount)

		var (
			historyID   uuid.UUID
			fromStatus  *string
			toStatus    string
			historyTime time.Time
		)

		err = testPool.QueryRow(
			ctx,
			`
				SELECT id, from_status, to_status, created_at
				FROM trip_history
				WHERE trip_id = $1
			`,
			got.ID,
		).Scan(
			&historyID,
			&fromStatus,
			&toStatus,
			&historyTime,
		)
		require.NoError(t, err)

		require.NotEqual(t, uuid.Nil, historyID)
		require.Nil(t, fromStatus)
		require.Equal(t, string(domain.TripStatusDraft), toStatus)
		require.False(t, historyTime.IsZero())
	})
}

func TestServer_CreateTrip_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		change        func(*dto.CreateTripRequest)
		errorContains string
	}{
		{
			name: "время отправления в прошлом",
			change: func(r *dto.CreateTripRequest) {
				r.DepartureTime = time.Now().
					Add(-time.Hour).
					Format(time.RFC3339)
			},
			errorContains: "недопустимое время начало поездки",
		},
		{
			name: "отрицательное количество мест",
			change: func(r *dto.CreateTripRequest) {
				r.AvailableSeats = -1
			},
			errorContains: "недопустимое кол-во слотов в поездке",
		},
		{
			name: "нулевое количество мест",
			change: func(r *dto.CreateTripRequest) {
				r.AvailableSeats = 0
			},
		},
		{
			name: "пустой client_id",
			change: func(r *dto.CreateTripRequest) {
				r.ClientID = ""
			},
		},
		{
			name: "некорректный UUID",
			change: func(r *dto.CreateTripRequest) {
				r.ClientID = "not-a-uuid"
			},
		},
		{
			name: "пустой пункт отправления",
			change: func(r *dto.CreateTripRequest) {
				r.FromPoint = ""
			},
		},
		{
			name: "пункт отправления из пробелов",
			change: func(r *dto.CreateTripRequest) {
				r.FromPoint = "   "
			},
		},
		{
			name: "пустой пункт назначения",
			change: func(r *dto.CreateTripRequest) {
				r.ToPoint = ""
			},
		},
		{
			name: "пункт назначения из пробелов",
			change: func(r *dto.CreateTripRequest) {
				r.ToPoint = "   "
			},
		},
		{
			name: "пустая дата",
			change: func(r *dto.CreateTripRequest) {
				r.DepartureTime = ""
			},
		},
		{
			name: "некорректный формат даты",
			change: func(r *dto.CreateTripRequest) {
				r.DepartureTime = "завтра"
			},
		},
		{
			name: "несуществующая дата",
			change: func(r *dto.CreateTripRequest) {
				r.DepartureTime = "2030-02-30T12:00:00Z"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Уникальные пункты позволяют найти возможную ошибочную
			// запись, даже если тест изменяет client_id.
			fromMarker := "Москва-" + uuid.NewString()
			toMarker := "Владивосток-" + uuid.NewString()

			payload := dto.CreateTripRequest{
				ClientID:       uuid.NewString(),
				FromPoint:      fromMarker,
				ToPoint:        toMarker,
				DepartureTime:  time.Now().Add(time.Hour).Format(time.RFC3339),
				AvailableSeats: 1,
			}

			// Каждый кейс меняет только одно поле.
			// Поэтому хотя бы одна уникальная метка сохраняется.
			tt.change(&payload)

			response := sendCreateTripRequest(t, payload)

			require.NotEmpty(t, response.Errors)
			require.Nil(t, response.Data)

			if tt.errorContains != "" {
				require.Contains(
					t,
					strings.Join(response.Errors, "; "),
					tt.errorContains,
				)
			}

			ctx, cancel := context.WithTimeout(
				context.Background(),
				2*time.Second,
			)
			defer cancel()

			var count int

			err := testPool.QueryRow(
				ctx,
				`
					SELECT count(*)
					FROM trips
					WHERE from_point = $1 OR to_point = $2
				`,
				fromMarker,
				toMarker,
			).Scan(&count)
			require.NoError(t, err)
			require.Zero(
				t,
				count,
				"невалидная поездка не должна сохраняться",
			)
		})
	}
}
