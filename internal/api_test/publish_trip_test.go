package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api/dto"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/stretchr/testify/require"
)

type publishTripTestResponse struct {
	Data   *dto.PublishTripResponse `json:"data"`
	Errors []string                 `json:"errors"`
}

type publishTripDBState struct {
	Status       string
	HistoryCount int
	EventCount   int
}

func sendPublishTripRequest(
	t *testing.T,
	payload dto.PublishTripRequest,
) publishTripTestResponse {
	t.Helper()

	return sendJSONRequest[publishTripTestResponse](
		t,
		http.MethodPost,
		tripTestURL+"publish",
		payload,
		http.StatusOK,
	)
}

func createDraftForPublishTest(t *testing.T) *dto.CreateTripResponse {
	t.Helper()

	response := sendCreateTripRequest(t, dto.CreateTripRequest{
		ClientID:       uuid.NewString(),
		FromPoint:      "Москва",
		ToPoint:        "Коломна",
		DepartureTime:  time.Now().Add(time.Hour).Format(time.RFC3339),
		AvailableSeats: 2,
	})

	require.Empty(t, response.Errors)
	require.NotNil(t, response.Data)
	require.Equal(
		t,
		string(domain.TripStatusDraft),
		response.Data.Status,
	)

	return response.Data
}

func readPublishTripState(
	t *testing.T,
	tripID uuid.UUID,
) publishTripDBState {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var state publishTripDBState

	err := testPool.QueryRow(
		ctx,
		`
			SELECT
				t.status,
				(SELECT count(*)
				 FROM trip_history
				 WHERE trip_id = t.id),
				(SELECT count(*)
				 FROM outbox_event
				 WHERE aggregate_id = t.id)
			FROM trips AS t
			WHERE t.id = $1
		`,
		tripID,
	).Scan(
		&state.Status,
		&state.HistoryCount,
		&state.EventCount,
	)
	require.NoError(t, err)

	return state
}

func setTripStatusForPublishTest(
	t *testing.T,
	tripID uuid.UUID,
	status domain.TripStatus,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Подготавливаем состояние для конкретного теста.
	tag, err := testPool.Exec(
		ctx,
		`UPDATE trips SET status = $1 WHERE id = $2`,
		string(status),
		tripID,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, tag.RowsAffected())
}

func TestServer_PublishTrip(t *testing.T) {
	t.Parallel()
	t.Run("публикация создаёт историю и событие", func(t *testing.T) {
		t.Parallel()
		trip := createDraftForPublishTest(t)
		before := readPublishTripState(t, trip.ID)

		response := sendPublishTripRequest(t, dto.PublishTripRequest{
			TripID:   trip.ID.String(),
			ClientID: trip.ClientID.String(),
		})

		require.Empty(t, response.Errors)
		require.NotNil(t, response.Data)
		require.Equal(t, trip.ID, response.Data.TripID)

		after := readPublishTripState(t, trip.ID)

		require.Equal(t, string(domain.TripStatusPublished), after.Status)
		require.Equal(t, before.HistoryCount+1, after.HistoryCount)
		require.Equal(t, before.EventCount+1, after.EventCount)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Проверяем переход в истории.
		var (
			historyID   uuid.UUID
			fromStatus  *string
			toStatus    string
			historyTime time.Time
		)

		err := testPool.QueryRow(
			ctx,
			`
				SELECT id, from_status, to_status, created_at
				FROM trip_history
				WHERE trip_id = $1 AND to_status = $2
			`,
			trip.ID,
			string(domain.TripStatusPublished),
		).Scan(
			&historyID,
			&fromStatus,
			&toStatus,
			&historyTime,
		)
		require.NoError(t, err)

		require.NotEqual(t, uuid.Nil, historyID)
		require.NotNil(t, fromStatus)
		require.Equal(t, string(domain.TripStatusDraft), *fromStatus)
		require.Equal(t, string(domain.TripStatusPublished), toStatus)
		require.False(t, historyTime.IsZero())

		// Проверяем содержимое outbox-события.
		var (
			eventID     uuid.UUID
			eventName   string
			aggregateID uuid.UUID
			payload     []byte
			eventTime   time.Time
		)

		err = testPool.QueryRow(
			ctx,
			`
				SELECT id, event_name, aggregate_id, payload, created_at
				FROM outbox_event
				WHERE aggregate_id = $1 AND event_name = $2
			`,
			trip.ID,
			"trip_published",
		).Scan(
			&eventID,
			&eventName,
			&aggregateID,
			&payload,
			&eventTime,
		)
		require.NoError(t, err)

		require.NotEqual(t, uuid.Nil, eventID)
		require.Equal(t, "trip_published", eventName)
		require.Equal(t, trip.ID, aggregateID)
		require.False(t, eventTime.IsZero())
		require.True(t, historyTime.Equal(eventTime))

		var eventPayload struct {
			TripID uuid.UUID `json:"trip_id"`
		}

		require.NoError(t, json.Unmarshal(payload, &eventPayload))
		require.Equal(t, trip.ID, eventPayload.TripID)
	})

	t.Run("повторная публикация не создаёт дубликаты", func(t *testing.T) {
		t.Parallel()
		trip := createDraftForPublishTest(t)

		request := dto.PublishTripRequest{
			TripID:   trip.ID.String(),
			ClientID: trip.ClientID.String(),
		}

		first := sendPublishTripRequest(t, request)
		require.Empty(t, first.Errors)
		require.NotNil(t, first.Data)
		require.Equal(t, trip.ID, first.Data.TripID)

		before := readPublishTripState(t, trip.ID)
		require.Equal(t, string(domain.TripStatusPublished), before.Status)

		second := sendPublishTripRequest(t, request)
		require.Empty(t, second.Errors)
		require.NotNil(t, second.Data)
		require.Equal(t, trip.ID, second.Data.TripID)

		after := readPublishTripState(t, trip.ID)
		require.Equal(t, before, after)
	})

	t.Run("поездка не найдена", func(t *testing.T) {
		t.Parallel()
		tripID := uuid.New()

		response := sendPublishTripRequest(t, dto.PublishTripRequest{
			TripID:   tripID.String(),
			ClientID: uuid.NewString(),
		})

		require.Nil(t, response.Data)
		require.NotEmpty(t, response.Errors)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var tripCount, historyCount, eventCount int

		err := testPool.QueryRow(
			ctx,
			`
				SELECT
					(SELECT count(*) FROM trips WHERE id = $1),
					(SELECT count(*) FROM trip_history WHERE trip_id = $1),
					(SELECT count(*) FROM outbox_event WHERE aggregate_id = $1)
			`,
			tripID,
		).Scan(&tripCount, &historyCount, &eventCount)
		require.NoError(t, err)

		require.Zero(t, tripCount)
		require.Zero(t, historyCount)
		require.Zero(t, eventCount)
	})
}

func TestServer_PublishTrip_DomainErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		status        domain.TripStatus
		anotherClient bool
		errorContains string
	}{
		{
			name:          "чужой клиент не может опубликовать черновик",
			status:        domain.TripStatusDraft,
			anotherClient: true,
			errorContains: "forbidden",
		},
		{
			name:          "чужой клиент не получает успех для опубликованной поездки",
			status:        domain.TripStatusPublished,
			anotherClient: true,
			errorContains: "forbidden",
		},
		{
			name:          "отменённую поездку нельзя опубликовать",
			status:        domain.TripStatusCanceled,
			errorContains: "invalid trip status",
		},
		{
			name:          "завершённую поездку нельзя опубликовать",
			status:        domain.TripStatusCompleted,
			errorContains: "invalid trip status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			trip := createDraftForPublishTest(t)
			setTripStatusForPublishTest(t, trip.ID, tt.status)

			before := readPublishTripState(t, trip.ID)

			clientID := trip.ClientID
			if tt.anotherClient {
				clientID = uuid.New()
			}

			response := sendPublishTripRequest(t, dto.PublishTripRequest{
				TripID:   trip.ID.String(),
				ClientID: clientID.String(),
			})

			require.Nil(t, response.Data)
			require.NotEmpty(t, response.Errors)
			require.Contains(
				t,
				strings.Join(response.Errors, "; "),
				tt.errorContains,
			)

			after := readPublishTripState(t, trip.ID)
			require.Equal(
				t,
				before,
				after,
				"отказ не должен менять поездку, историю и outbox",
			)
		})
	}
}

func TestServer_PublishTrip_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*dto.PublishTripRequest)
	}{
		{
			name: "пустой trip_id",
			change: func(r *dto.PublishTripRequest) {
				r.TripID = ""
			},
		},
		{
			name: "некорректный trip_id",
			change: func(r *dto.PublishTripRequest) {
				r.TripID = "not-a-uuid"
			},
		},
		{
			name: "пустой client_id",
			change: func(r *dto.PublishTripRequest) {
				r.ClientID = ""
			},
		},
		{
			name: "некорректный client_id",
			change: func(r *dto.PublishTripRequest) {
				r.ClientID = "not-a-uuid"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			trip := createDraftForPublishTest(t)
			before := readPublishTripState(t, trip.ID)

			request := dto.PublishTripRequest{
				TripID:   trip.ID.String(),
				ClientID: trip.ClientID.String(),
			}
			tt.change(&request)

			response := sendPublishTripRequest(t, request)

			require.Nil(t, response.Data)
			require.NotEmpty(t, response.Errors)

			after := readPublishTripState(t, trip.ID)
			require.Equal(t, before, after)
		})
	}
}
