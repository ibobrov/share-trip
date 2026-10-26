package contract

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestClient_RetryPreservesProcessContext(t *testing.T) {
	t.Parallel()
	requests := make(chan http.Header, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data":null,"errors":["temporarily unavailable"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"allowed":true},"errors":[]}`))
	}))
	t.Cleanup(server.Close)
	observer := metrics.New(prometheus.NewRegistry())
	client, err := NewClient(server.URL, server.Client(), DefaultClientConfig(), observer)
	require.NoError(t, err)
	ctx := logctx.WithRequestID(t.Context(), "request-123")
	ctx = logctx.WithCorrelationID(ctx, "publication-456")
	ctx = logctx.WithBusinessIDs(ctx, "trip-789", "user-123")
	ctx = propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})
	response, err := client.CheckOption(ctx, "company-acme", "trip_publication")
	require.NoError(t, err)
	require.True(t, response.Allowed)
	require.Len(t, requests, 2)
	for range 2 {
		headers := <-requests
		require.Equal(t, "request-123", headers.Get("X-Request-ID"))
		require.Equal(t, "publication-456", headers.Get("X-Correlation-ID"))
		require.Equal(t, "trip-789", headers.Get("X-Trip-ID"))
		require.Equal(t, "user-123", headers.Get("X-User-ID"))
		remote := propagation.TraceContext{}.Extract(t.Context(), propagation.HeaderCarrier(headers))
		require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", trace.SpanContextFromContext(remote).TraceID().String())
	}
	// A permission check is one operation even when it needs two HTTP attempts.
	require.InDelta(t, float64(1), testutil.ToFloat64(observer.ContractRequestTotal.WithLabelValues("allowed")), 1e-9)
	require.Zero(t, testutil.ToFloat64(observer.ContractRequestTotal.WithLabelValues("error")))
}

func TestClientDoesNotExposeUpstreamErrorText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "error body", status: http.StatusInternalServerError, body: `{"data":null,"errors":["PRIVATE_TEST_VALUE"]}`},
		{name: "unknown denial reason", status: http.StatusOK, body: `{"data":{"allowed":false,"reason":"PRIVATE_TEST_VALUE"},"errors":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)
			cfg := DefaultClientConfig()
			cfg.RetryAttempts = 1
			client, err := NewClient(server.URL, server.Client(), cfg, nil)
			require.NoError(t, err)
			var logs bytes.Buffer
			ctx := logctx.WithLogger(t.Context(), slog.New(slog.NewJSONHandler(&logs, nil)))

			_, err = client.CheckOption(ctx, "company-acme", "trip_creation")

			require.Error(t, err)
			require.NotContains(t, err.Error(), "PRIVATE_TEST_VALUE")
			require.NotContains(t, logs.String(), "PRIVATE_TEST_VALUE")
			require.Contains(t, logs.String(), `"level":"ERROR"`)
		})
	}
}
