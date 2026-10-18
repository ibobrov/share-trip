package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/api"
	contractgen "github.com/ibobrov/share_trip/internal/clients/contract/gen"
	"github.com/stretchr/testify/require"
)

func TestServer_CreateTrip_ContractOption(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		contractStatus int
		contractBody   string
		wantStatus     int
		wantError      string
	}
	tests := []testCase{
		{
			name:           "active option allows trip creation",
			contractStatus: http.StatusOK,
			contractBody:   `{"data":{"allowed":true},"errors":[]}`,
			wantStatus:     http.StatusOK,
		},
		{
			name:           "contract service error prevents trip creation",
			contractStatus: http.StatusInternalServerError,
			contractBody:   `{"data":null,"errors":["internal error"]}`,
			wantStatus:     http.StatusInternalServerError,
			wantError:      "check trip_creation availability",
		},
	}
	for _, reason := range []string{
		"CONTRACT_NOT_FOUND", "NO_ACTIVE_CONTRACT", "CONTRACT_NOT_STARTED",
		"CONTRACT_EXPIRED", "SERVICE_NOT_ALLOWED",
	} {
		tests = append(tests, testCase{
			name:           "denied: " + reason,
			contractStatus: http.StatusOK,
			contractBody:   fmt.Sprintf(`{"data":{"allowed":false,"reason":%q},"errors":[]}`, reason),
			wantStatus:     http.StatusForbidden,
			wantError:      reason,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			companyID := "company-" + uuid.NewString()
			contractResponses.Store(companyID, contractTestResponse{status: tt.contractStatus, body: tt.contractBody})
			t.Cleanup(func() {
				contractResponses.Delete(companyID)
				contractRequests.Delete(companyID)
			})

			response := sendCreateTripRequest(t, tt.wantStatus, api.CreateTripRequest{
				FromPoint:      "Москва",
				ToPoint:        "Коломна",
				DepartureTime:  time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
				AvailableSeats: 1,
			}, withUser(uuid.New()), withCompany(companyID))

			request, called := contractRequests.Load(companyID)
			require.True(t, called, "company from token must be checked in Contract Service")
			require.Equal(t, contractgen.CheckOptionRequest{
				CompanyID: companyID, OptionCode: contractgen.TripCreation,
			}, request)

			var tripCount, historyCount int
			err := testPool.QueryRow(t.Context(), `SELECT count(*) FROM trips WHERE company_id = $1`, companyID).Scan(&tripCount)
			require.NoError(t, err)
			err = testPool.QueryRow(t.Context(), `SELECT count(*) FROM trip_history h JOIN trips t ON t.id = h.trip_id WHERE t.company_id = $1`, companyID).Scan(&historyCount)
			require.NoError(t, err)

			if tt.wantError != "" {
				require.Nil(t, response.Data)
				require.Len(t, response.Errors, 1)
				require.Contains(t, response.Errors[0], tt.wantError)
				require.Zero(t, tripCount)
				require.Zero(t, historyCount)
				return
			}
			require.Empty(t, response.Errors)
			require.NotNil(t, response.Data)
			require.Equal(t, companyID, response.Data.CompanyID)
			require.Equal(t, 1, tripCount)
			require.Equal(t, 1, historyCount)
		})
	}
}
