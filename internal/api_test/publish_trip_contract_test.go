package api_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api"
	contractgen "github.com/ibobrov/share_trip/internal/clients/contract/gen"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestServer_PublishTrip_ContractOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		contractStatus int
		body           string
		wantStatus     int
		wantError      string
	}{
		{
			name: "allowed", contractStatus: http.StatusOK,
			body: `{"data":{"allowed":true},"errors":[]}`, wantStatus: http.StatusCreated,
		},
		{
			name: "contract expired after draft creation", contractStatus: http.StatusOK,
			body:       `{"data":{"allowed":false,"reason":"CONTRACT_EXPIRED"},"errors":[]}`,
			wantStatus: http.StatusForbidden, wantError: "CONTRACT_EXPIRED",
		},
		{
			name: "option disabled after draft creation", contractStatus: http.StatusOK,
			body:       `{"data":{"allowed":false,"reason":"SERVICE_NOT_ALLOWED"},"errors":[]}`,
			wantStatus: http.StatusForbidden, wantError: "SERVICE_NOT_ALLOWED",
		},
		{
			name: "contract service error", contractStatus: http.StatusInternalServerError,
			body:       `{"data":null,"errors":["internal error"]}`,
			wantStatus: http.StatusInternalServerError, wantError: "contract.CheckOption: HTTP 500",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			clientID := uuid.New()
			trip := createDraftForPublishTest(t, clientID)
			before := readPublishTripState(t, trip.ID)
			contractRequests.Delete(trip.CompanyID)
			contractResponses.Store(trip.CompanyID, contractTestResponse{status: tt.contractStatus, body: tt.body})
			t.Cleanup(func() {
				contractResponses.Delete(trip.CompanyID)
				contractRequests.Delete(trip.CompanyID)
			})

			response := sendPublishTripRequest(t, tt.wantStatus, api.PublishTripRequest{TripID: trip.ID.String()},
				withUser(clientID), withCompany("another-company-from-token"))
			request, called := contractRequests.Load(trip.CompanyID)
			require.True(t, called, "check company saved in trip rather than company in current token")
			require.Equal(t, contractgen.CheckOptionRequest{CompanyID: trip.CompanyID, OptionCode: contractgen.TripPublication}, request)
			after := readPublishTripState(t, trip.ID)
			if tt.wantError != "" {
				require.Nil(t, response.Data)
				require.Len(t, response.Errors, 1)
				require.Contains(t, response.Errors[0], tt.wantError)
				require.Equal(t, before, after, "denial must not change trip, history or outbox")
				require.Empty(t, queuedEvents(t, trip.ID))
				return
			}
			require.Empty(t, response.Errors)
			require.NotNil(t, response.Data)
			require.Equal(t, trip.ID, response.Data.TripID)
			require.Equal(t, string(domain.TripStatusPublished), after.Status)
			require.Equal(t, before.HistoryCount+1, after.HistoryCount)
			require.Equal(t, before.EventCount+1, after.EventCount)
			requireQueuedEvent(t, trip.ID, clientID, trip.CompanyID)
		})
	}
}

func TestServer_PublishTrip_AlreadyPublishedWithoutContract(t *testing.T) {
	t.Parallel()
	clientID := uuid.New()
	trip := createDraftForPublishTest(t, clientID)
	request := api.PublishTripRequest{TripID: trip.ID.String()}
	first := sendPublishTripRequest(t, http.StatusCreated, request, withUser(clientID))
	require.Empty(t, first.Errors)
	before := readPublishTripState(t, trip.ID)
	contractRequests.Delete(trip.CompanyID)
	contractResponses.Store(trip.CompanyID, contractTestResponse{
		status: http.StatusInternalServerError, body: `{"data":null,"errors":["unavailable"]}`,
	})
	t.Cleanup(func() {
		contractResponses.Delete(trip.CompanyID)
		contractRequests.Delete(trip.CompanyID)
	})

	response := sendPublishTripRequest(t, http.StatusOK, request, withUser(clientID))
	require.Empty(t, response.Errors)
	require.NotNil(t, response.Data)
	require.Equal(t, trip.ID, response.Data.TripID)
	_, called := contractRequests.Load(trip.CompanyID)
	require.False(t, called)
	require.Equal(t, before, readPublishTripState(t, trip.ID))
}

func TestServer_PublishTrip_ContractDeniedWithoutRowLock(t *testing.T) {
	t.Parallel()
	clientID := uuid.New()
	trip := createDraftForPublishTest(t, clientID)
	before := readPublishTripState(t, trip.ID)
	contractResponses.Store(trip.CompanyID, contractTestResponse{
		status: http.StatusOK,
		body:   `{"data":{"allowed":false,"reason":"SERVICE_NOT_ALLOWED"},"errors":[]}`,
	})
	t.Cleanup(func() {
		contractResponses.Delete(trip.CompanyID)
		contractRequests.Delete(trip.CompanyID)
	})

	// Another transaction owns the row lock. The preliminary SELECT must not wait for it.
	lockTx, err := testPool.Begin(t.Context())
	require.NoError(t, err)
	defer func() { require.NoError(t, lockTx.Rollback(t.Context())) }()
	var lockedID uuid.UUID
	err = lockTx.QueryRow(t.Context(), `SELECT id FROM trips WHERE id = $1 FOR UPDATE`, trip.ID).Scan(&lockedID)
	require.NoError(t, err)
	require.Equal(t, trip.ID, lockedID)

	response := sendPublishTripRequest(t, http.StatusForbidden,
		api.PublishTripRequest{TripID: trip.ID.String()}, withUser(clientID))
	require.Nil(t, response.Data)
	require.Len(t, response.Errors, 1)
	require.Contains(t, response.Errors[0], "SERVICE_NOT_ALLOWED")
	require.Equal(t, before, readPublishTripState(t, trip.ID))
}
