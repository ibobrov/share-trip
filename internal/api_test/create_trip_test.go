package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/stretchr/testify/require"
)

type createTripTestResponse struct {
	Data   *api.CreateTripResponse `json:"data"`
	Errors []string                `json:"errors"`
}

func sendCreateTripRequest(
	t *testing.T,
	expectedStatus int,
	payload api.CreateTripRequest,
	options ...requestOption,
) createTripTestResponse {
	t.Helper()

	return sendJSONRequest[createTripTestResponse](
		t,
		http.MethodPost,
		tripTestURL+"create",
		payload,
		expectedStatus,
		options...,
	)
}

func TestServer_CreateTrip(t *testing.T) {
	t.Run("успешное создание поездки, истории", func(t *testing.T) {
		t.Parallel()

		clientID := uuid.New()
		companyID := "company-" + uuid.NewString()
		fromPoint := "Москва"
		toPoint := "Владивосток"
		departureTime := time.Now().
			UTC().
			Add(time.Hour).
			Truncate(time.Second)
		availableSeats := 1

		res := sendCreateTripRequest(t,
			fiber.StatusOK,
			api.CreateTripRequest{
				FromPoint:      fromPoint,
				ToPoint:        toPoint,
				DepartureTime:  departureTime.Format(time.RFC3339),
				AvailableSeats: availableSeats,
			},
			withUser(clientID),
			withCompany(companyID),
		)

		require.Empty(t, res.Errors)
		require.NotNil(t, res.Data)

		got := res.Data

		require.NotEqual(t, uuid.Nil, got.ID)
		require.Equal(t, clientID, got.ClientID)
		require.Equal(t, companyID, got.CompanyID)
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
		require.Equal(t, string(domain.TripStatusDraft), got.Status)
		require.False(t, got.CreatedAt.IsZero())

		ctx := context.Background()

		// Проверяем фактическое сохранение поездки.
		var (
			savedClientID      uuid.UUID
			savedCompanyID     string
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
					company_id,
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
			&savedCompanyID,
			&savedFromPoint,
			&savedToPoint,
			&savedDepartureTime,
			&savedSeats,
			&savedStatus,
		)
		require.NoError(t, err)

		require.Equal(t, clientID, savedClientID)
		require.Equal(t, companyID, savedCompanyID)
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
		change        func(*api.CreateTripRequest)
		errorContains string
	}{
		{
			name: "время отправления в прошлом",
			change: func(r *api.CreateTripRequest) {
				r.DepartureTime = time.Now().
					Add(-time.Hour).
					Format(time.RFC3339)
			},
			errorContains: "недопустимое время начало поездки",
		},
		{
			name: "отрицательное количество мест",
			change: func(r *api.CreateTripRequest) {
				r.AvailableSeats = -1
			},
			errorContains: "недопустимое кол-во слотов в поездке",
		},
		{
			name: "нулевое количество мест",
			change: func(r *api.CreateTripRequest) {
				r.AvailableSeats = 0
			},
		},
		{
			name: "пустой пункт отправления",
			change: func(r *api.CreateTripRequest) {
				r.FromPoint = ""
			},
		},
		{
			name: "пункт отправления из пробелов",
			change: func(r *api.CreateTripRequest) {
				r.FromPoint = "   "
			},
		},
		{
			name: "пустой пункт назначения",
			change: func(r *api.CreateTripRequest) {
				r.ToPoint = ""
			},
		},
		{
			name: "пункт назначения из пробелов",
			change: func(r *api.CreateTripRequest) {
				r.ToPoint = "   "
			},
		},
		{
			name: "пустая дата",
			change: func(r *api.CreateTripRequest) {
				r.DepartureTime = ""
			},
		},
		{
			name: "некорректный формат даты",
			change: func(r *api.CreateTripRequest) {
				r.DepartureTime = "завтра"
			},
		},
		{
			name: "несуществующая дата",
			change: func(r *api.CreateTripRequest) {
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
			clientId := uuid.New()

			payload := api.CreateTripRequest{
				FromPoint:      fromMarker,
				ToPoint:        toMarker,
				DepartureTime:  time.Now().Add(time.Hour).Format(time.RFC3339),
				AvailableSeats: 1,
			}

			// Каждый кейс меняет только одно поле.
			// Поэтому хотя бы одна уникальная метка сохраняется.
			tt.change(&payload)

			response := sendCreateTripRequest(t, fiber.StatusOK, payload, withUser(clientId))

			require.NotEmpty(t, response.Errors)
			require.Nil(t, response.Data)

			if tt.errorContains != "" {
				require.Contains(
					t,
					strings.Join(response.Errors, "; "),
					tt.errorContains,
				)
			}

			ctx := context.Background()

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

func TestServer_CreateTrip_CompanyFromToken(t *testing.T) {
	t.Parallel()

	clientID := uuid.New()
	companyID := "company-from-token-" + uuid.NewString()

	// Поле компании в HTTP-теле не должно подменять claim токена.
	payload := struct {
		CompanyID      string `json:"company_id"`
		FromPoint      string `json:"from_point"`
		ToPoint        string `json:"to_point"`
		DepartureTime  string `json:"departure_time"`
		AvailableSeats int    `json:"available_seats"`
	}{
		CompanyID:      "company-from-body",
		FromPoint:      "Москва",
		ToPoint:        "Коломна",
		DepartureTime:  time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		AvailableSeats: 1,
	}

	response := sendJSONRequest[createTripTestResponse](
		t,
		http.MethodPost,
		tripTestURL+"create",
		payload,
		fiber.StatusOK,
		withUser(clientID),
		withCompany(companyID),
	)

	require.Empty(t, response.Errors)
	require.NotNil(t, response.Data)
	require.Equal(t, companyID, response.Data.CompanyID)

	var savedCompanyID string
	err := testPool.QueryRow(
		context.Background(),
		`SELECT company_id FROM trips WHERE id = $1`,
		response.Data.ID,
	).Scan(&savedCompanyID)
	require.NoError(t, err)
	require.Equal(t, companyID, savedCompanyID)
}

func TestServer_CreateTrip_MissingCompanyClaim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		companyID string
	}{
		{name: "company_id отсутствует", companyID: ""},
		{name: "company_id содержит только пробелы", companyID: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fromPoint := "Москва-" + uuid.NewString()
			response := sendCreateTripRequest(t,
				fiber.StatusForbidden,
				api.CreateTripRequest{
					FromPoint:      fromPoint,
					ToPoint:        "Коломна",
					DepartureTime:  time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
					AvailableSeats: 1,
				},
				withUser(uuid.New()),
				withCompany(tt.companyID),
			)

			require.Nil(t, response.Data)
			require.Equal(t, []string{"company_id is required in token"}, response.Errors)

			var tripCount int
			err := testPool.QueryRow(
				context.Background(),
				`SELECT count(*) FROM trips WHERE from_point = $1`,
				fromPoint,
			).Scan(&tripCount)
			require.NoError(t, err)
			require.Zero(t, tripCount, "без компании в токене поездка не должна сохраняться")
		})
	}
}
