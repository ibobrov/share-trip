package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"

	contractgen "github.com/ibobrov/share_trip/internal/clients/contract/gen"
)

type contractTestResponse struct {
	status int
	body   string
}

var contractResponses sync.Map
var contractRequests sync.Map

func newContractTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/api/option/check" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var request contractgen.CheckOptionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || (request.OptionCode != contractgen.TripCreation && request.OptionCode != contractgen.TripPublication) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"data":null,"errors":["invalid option check request"]}`))
			return
		}
		contractRequests.Store(request.CompanyID, request)

		response := contractTestResponse{
			status: http.StatusOK,
			body:   `{"data":{"allowed":true},"errors":[]}`,
		}
		if configured, ok := contractResponses.Load(request.CompanyID); ok {
			response = configured.(contractTestResponse)
		}
		w.WriteHeader(response.status)
		_, _ = w.Write([]byte(response.body))
	}))
}
