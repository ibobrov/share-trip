package contract

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	contractgen "github.com/ibobrov/share_trip/internal/clients/contract/gen"
	"github.com/stretchr/testify/require"
)

func TestClient_CheckOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		want       CheckResult
		wantError  string
	}{
		{
			name:       "option is allowed without reason",
			statusCode: http.StatusOK,
			body:       `{"data":{"allowed":true},"errors":[]}`,
			want:       CheckResult{Allowed: true},
		},
		{
			name:       "option is denied with reason",
			statusCode: http.StatusOK,
			body:       `{"data":{"allowed":false,"reason":"CONTRACT_EXPIRED"},"errors":[]}`,
			want:       CheckResult{Allowed: false, Reason: "CONTRACT_EXPIRED"},
		},
		{
			name:       "validation error",
			statusCode: http.StatusBadRequest,
			body:       `{"data":null,"errors":["company_id is required"]}`,
			wantError:  "HTTP 400: company_id is required",
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"data":null,"errors":["internal error"]}`,
			wantError:  "HTTP 500: internal error",
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
			client := &Client{generated: generated}

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
			client, err := NewClient(server.URL, server.Client())
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
	client, err := NewClient(server.URL, server.Client())
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
	client, err := NewClient(server.URL, server.Client())
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
	require.ErrorContains(t, err, "invalid option code: unknown")
}
