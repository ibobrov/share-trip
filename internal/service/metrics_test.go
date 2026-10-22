package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ibobrov/share_trip/internal/clients/contract"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

type contractCheckerStub struct {
	result contract.CheckResult
	err    error
}

func (s contractCheckerStub) CheckOption(context.Context, string, string) (contract.CheckResult, error) {
	return s.result, s.err
}

func TestTripService_CreateTrip_Metrics(t *testing.T) {
	t.Parallel()

	contractError := errors.New("contract service unavailable")
	tests := []struct {
		name       string
		checker    contractCheckerStub
		wantResult string
		wantError  error
	}{
		{
			name:       "contract denial is forbidden even though client returned no error",
			checker:    contractCheckerStub{result: contract.CheckResult{Reason: "CONTRACT_EXPIRED"}},
			wantResult: "forbidden",
			wantError:  domain.ErrForbidden,
		},
		{
			name:       "contract client error is internal error",
			checker:    contractCheckerStub{err: contractError},
			wantResult: "internal_error",
			wantError:  contractError,
		},
		{
			name:       "begin transaction failure is internal error instead of success",
			checker:    contractCheckerStub{result: contract.CheckResult{Allowed: true}},
			wantResult: "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A closed pool fails BEGIN without requiring a running database.
			pool, err := pgxpool.New(t.Context(), "postgres://localhost/test")
			require.NoError(t, err)
			pool.Close()
			operationMetrics := metrics.New(prometheus.NewRegistry())
			service := NewTripService(pool, nil, operationMetrics, tt.checker)

			_, err = service.CreateTrip(t.Context(), CreateTripRequest{CompanyID: "company-acme"})
			require.Error(t, err)
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
			}

			var counter dto.Metric
			err = operationMetrics.TripCreateTotal.WithLabelValues(tt.wantResult).Write(&counter)
			require.NoError(t, err)
			require.InDelta(t, 1, counter.GetCounter().GetValue(), 0.001)
			err = operationMetrics.TripCreateTotal.WithLabelValues("success").Write(&counter)
			require.NoError(t, err)
			require.Zero(t, counter.GetCounter().GetValue())
		})
	}
}

func TestTripService_PublishTrip_BeginFailureMetrics(t *testing.T) {
	t.Parallel()

	pool, err := pgxpool.New(t.Context(), "postgres://localhost/test")
	require.NoError(t, err)
	pool.Close()
	operationMetrics := metrics.New(prometheus.NewRegistry())
	service := NewTripService(pool, nil, operationMetrics, nil)

	_, err = service.PublishTrip(t.Context(), PublishTripRequest{})
	require.Error(t, err)

	var counter dto.Metric
	err = operationMetrics.TripPublishTotal.WithLabelValues("internal_error").Write(&counter)
	require.NoError(t, err)
	require.InDelta(t, 1, counter.GetCounter().GetValue(), 0.001)
}
