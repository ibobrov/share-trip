package service_test

import (
	"errors"
	"testing"

	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/ibobrov/share_trip/internal/service/mocks"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTripService_CreateTrip_Metrics(t *testing.T) {
	t.Parallel()

	contractError := errors.New("contract service unavailable")
	tests := []struct {
		name        string
		checkResult service.CheckResult
		checkError  error
		wantResult  string
		wantError   error
	}{
		{
			name:        "contract denial is forbidden even though client returned no error",
			checkResult: service.CheckResult{Reason: "CONTRACT_EXPIRED"},
			wantResult:  "forbidden",
			wantError:   domain.ErrForbidden,
		},
		{
			name:       "contract client error is internal error",
			checkError: contractError,
			wantResult: "internal_error",
			wantError:  contractError,
		},
		{
			name:        "begin transaction failure is internal error instead of success",
			checkResult: service.CheckResult{Allowed: true},
			wantResult:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			pool := mocks.NewMockTransactionBeginner(ctrl)
			checker := mocks.NewMockContractChecker(ctrl)
			checker.EXPECT().CheckOption(gomock.Any(), "company-acme", "trip_creation").
				Return(tt.checkResult, tt.checkError).Times(1)
			if tt.checkResult.Allowed {
				pool.EXPECT().Begin(gomock.Any()).Return(nil, errors.New("begin failed")).Times(1)
			}
			operationMetrics := metrics.New(prometheus.NewRegistry())
			svc := service.NewTripService(pool, nil, operationMetrics, checker)

			_, err := svc.CreateTrip(t.Context(), service.CreateTripRequest{CompanyID: "company-acme"})
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

	ctrl := gomock.NewController(t)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	pool.EXPECT().Begin(gomock.Any()).Return(nil, pgx.ErrTxClosed).Times(1)
	operationMetrics := metrics.New(prometheus.NewRegistry())
	useCase := mocks.NewMockTripUseCase(ctrl)
	useCase.EXPECT().GetTripForPublication(gomock.Any(), domain.GetTripForPublicationRequest{}).
		Return(&domain.GetTripForPublicationResponse{CompanyID: "company-acme", Status: "draft"}, nil).Times(1)
	checker := mocks.NewMockContractChecker(ctrl)
	checker.EXPECT().CheckOption(gomock.Any(), "company-acme", "trip_publication").
		Return(service.CheckResult{Allowed: true}, nil).Times(1)
	svc := service.NewTripService(pool, useCase, operationMetrics, checker)

	_, err := svc.PublishTrip(t.Context(), service.PublishTripRequest{})
	require.Error(t, err)

	var counter dto.Metric
	err = operationMetrics.TripPublishTotal.WithLabelValues("internal_error").Write(&counter)
	require.NoError(t, err)
	require.InDelta(t, 1, counter.GetCounter().GetValue(), 0.001)
}
