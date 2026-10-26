package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/ibobrov/share_trip/internal/service/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"
)

//nolint:paralleltest // Replaces the process-wide tracer provider until this test returns.
func TestCreateTripSpanDistinguishesFailureFromBusinessDenial(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(previous)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, provider.Shutdown(ctx))
	}()
	ctrl := gomock.NewController(t)
	checker := mocks.NewMockContractChecker(ctrl)
	unavailable := errors.New("contract unavailable")
	gomock.InOrder(
		checker.EXPECT().CheckOption(gomock.Any(), "company-acme", "trip_creation").Return(service.CheckResult{}, unavailable),
		checker.EXPECT().CheckOption(gomock.Any(), "company-acme", "trip_creation").Return(service.CheckResult{Reason: "CONTRACT_EXPIRED"}, nil),
	)
	svc := service.NewTripService(nil, nil, metrics.New(prometheus.NewRegistry()), checker)

	_, err := svc.CreateTrip(t.Context(), service.CreateTripRequest{CompanyID: "company-acme"})
	require.ErrorIs(t, err, unavailable)
	_, err = svc.CreateTrip(t.Context(), service.CreateTripRequest{CompanyID: "company-acme"})
	require.ErrorIs(t, err, domain.ErrForbidden)

	spans := recorder.Ended()
	require.Len(t, spans, 2)
	require.Equal(t, "TripService.CreateTrip", spans[0].Name())
	require.Equal(t, codes.Error, spans[0].Status().Code)
	require.Len(t, spans[0].Events(), 1)
	require.Equal(t, codes.Unset, spans[1].Status().Code)
	require.Empty(t, spans[1].Events())
}
