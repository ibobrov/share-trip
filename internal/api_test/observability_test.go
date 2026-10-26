package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api"
	"github.com/ibobrov/share_trip/internal/clients/contract"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/observability/middleware"
	"github.com/ibobrov/share_trip/internal/repository"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestServer_PublishTrip_PersistsProcessMetadata(t *testing.T) {
	t.Parallel()
	owner := uuid.New()
	trip := createDraftForPublishTest(t, owner)
	sendPublishTripRequest(t, http.StatusCreated, api.PublishTripRequest{TripID: trip.ID.String()}, withUser(owner), func(request *http.Request) {
		request.Header.Set("X-Request-ID", "request-"+trip.ID.String())
		request.Header.Set("X-Correlation-ID", "publication-"+trip.ID.String())
	})
	var data []byte
	require.NoError(t, testPool.QueryRow(t.Context(), "SELECT payload FROM outbox_event WHERE aggregate_id=$1", trip.ID).Scan(&data))
	var payload entity.PublishTripPayload
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Equal(t, "request-"+trip.ID.String(), payload.RequestID)
	require.Equal(t, "publication-"+trip.ID.String(), payload.CorrelationID)
	require.Equal(t, payload.RequestID, payload.CausationID)
	require.Equal(t, owner, payload.ClientID)
	require.Equal(t, trip.ID, payload.TripID)
}

func TestServer_PublishTrip_LogsCommittedEventID(t *testing.T) {
	t.Parallel()
	owner := uuid.New()
	trip := createDraftForPublishTest(t, owner)
	contractServer := newContractTestServer()
	t.Cleanup(contractServer.Close)
	checker, err := contract.NewClient(contractServer.URL, contractServer.Client(), contract.DefaultClientConfig(), nil)
	require.NoError(t, err)
	registry := prometheus.NewRegistry()
	useCase := domain.NewTripUseCase(repository.NewTripRepository(testPool), repository.NewTripHistoryRepository(), repository.NewOutboxRepository())
	tripService := service.NewTripService(testPool, useCase, metrics.New(registry), checker)
	server := api.NewServer(testPool, tripService, registry)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil)).With("service", "sharetrip")
	app := fiber.New()
	app.Use(middleware.Correlation(logger))
	app.Use(testAuthMiddleware)
	app.Post("/api/trip/publish", server.PublishTrip)
	request := httptest.NewRequest(http.MethodPost, "/api/trip/publish", strings.NewReader(`{"trip_id":"`+trip.ID.String()+`"}`))
	request.Header.Set("Content-Type", "application/json")
	withUser(owner)(request)

	response, err := app.Test(request)

	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	require.Equal(t, http.StatusCreated, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"data":{"trip_id":"`+trip.ID.String()+`"},"errors":[]}`, string(body))
	var eventID uuid.UUID
	require.NoError(t, testPool.QueryRow(t.Context(), "SELECT id FROM outbox_event WHERE aggregate_id=$1", trip.ID).Scan(&eventID))
	var completion map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		if entry["operation"] == "PublishTrip" && entry["result"] == "success" {
			require.Nil(t, completion, "publication should have one final success log")
			completion = entry
		}
	}
	require.NotNil(t, completion)
	require.Equal(t, "INFO", completion["level"])
	require.Equal(t, trip.ID.String(), completion["trip_id"])
	require.Equal(t, eventID.String(), completion["event_id"])
}
