package contract

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	contractgen "github.com/ibobrov/share_trip/internal/clients/contract/gen"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/stretchr/testify/require"
)

func TestClient_CheckOption_ConfiguredRetryAttempts(t *testing.T) {
	t.Parallel()
	for _, attempts := range []int{1, 3} {
		t.Run(strconv.Itoa(attempts), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"data":null,"errors":["unavailable"]}`))
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.URL, server.Client(), ClientConfig{
				AttemptTimeout: 100 * time.Millisecond, CheckTimeout: time.Second,
				RetryAttempts: attempts, RetryDelay: time.Millisecond,
			}, nil)
			require.NoError(t, err)

			_, err = client.CheckOption(t.Context(), "company-acme", "trip_creation")
			require.ErrorContains(t, err, "HTTP 500")
			require.EqualValues(t, attempts, calls.Load())
		})
	}
}

func TestClient_CheckOption_Request(t *testing.T) {
	t.Parallel()

	type receivedRequest struct {
		method      string
		path        string
		contentType string
		body        string
		err         error
	}
	requests := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		requests <- receivedRequest{
			method: r.Method, path: r.URL.Path,
			contentType: r.Header.Get("Content-Type"), body: string(body), err: err,
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"allowed":true},"errors":[]}`))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, server.Client(), DefaultClientConfig(), nil)
	require.NoError(t, err)

	result, err := client.CheckOption(t.Context(), "company-acme", "trip_creation")
	require.NoError(t, err)
	require.True(t, result.Allowed)
	request := <-requests
	require.NoError(t, request.err)
	require.Equal(t, http.MethodPost, request.method)
	require.Equal(t, "/api/option/check", request.path)
	require.Equal(t, "application/json", request.contentType)
	require.JSONEq(t, `{"company_id":"company-acme","option_code":"trip_creation"}`, request.body)
}

func TestClient_CheckOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		want       service.CheckResult
		wantError  string
	}{
		{
			name:       "option is allowed without reason",
			statusCode: http.StatusOK,
			body:       `{"data":{"allowed":true},"errors":[]}`,
			want:       service.CheckResult{Allowed: true},
		},
		{
			name:       "option is denied with reason",
			statusCode: http.StatusOK,
			body:       `{"data":{"allowed":false,"reason":"CONTRACT_EXPIRED"},"errors":[]}`,
			want:       service.CheckResult{Allowed: false, Reason: "CONTRACT_EXPIRED"},
		},
		{
			name:       "validation error",
			statusCode: http.StatusBadRequest,
			body:       `{"data":null,"errors":["company_id is required"]}`,
			wantError:  "HTTP 400",
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"data":null,"errors":["internal error"]}`,
			wantError:  "HTTP 500",
		},
		{
			name:       "unexpected status",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"data":null,"errors":["unavailable"]}`,
			wantError:  "unexpected response, HTTP 503",
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       `{"data":`,
			wantError:  "unexpected end of JSON input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			generated, err := contractgen.NewClientWithResponses(server.URL)
			require.NoError(t, err)
			client := &Client{generated: generated, config: DefaultClientConfig()}

			result, err := client.CheckOption(t.Context(), "company-acme", "trip_creation")
			wantCalls := int32(1)
			if tt.statusCode == http.StatusInternalServerError {
				wantCalls = 2
			}
			require.Equal(t, wantCalls, calls.Load())
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestClient_CheckOption_Retry500(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"data":null,"errors":["internal error"]}`,
		`invalid json`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if calls.Add(1) == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(body))
					return
				}
				_, _ = w.Write([]byte(`{"data":{"allowed":true},"errors":[]}`))
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.URL, server.Client(), DefaultClientConfig(), nil)
			require.NoError(t, err)

			result, err := client.CheckOption(t.Context(), "company-acme", "trip_creation")
			require.NoError(t, err)
			require.True(t, result.Allowed)
			require.Equal(t, int32(2), calls.Load())
		})
	}
}

func TestClient_CheckOption_TimeoutIsNotRetried(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, server.Client(), DefaultClientConfig(), nil)
	require.NoError(t, err)

	_, err = client.CheckOption(t.Context(), "company-acme", "trip_creation")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int32(1), calls.Load())
}

func TestClient_CheckOption_CanceledContext(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, server.Client(), DefaultClientConfig(), nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = client.CheckOption(ctx, "company-acme", "trip_creation")
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, calls.Load())
}

func TestClient_CheckOption_InvalidCode(t *testing.T) {
	t.Parallel()

	client := &Client{}
	_, err := client.CheckOption(t.Context(), "company-acme", "unknown")
	require.ErrorContains(t, err, "invalid option code")
}
