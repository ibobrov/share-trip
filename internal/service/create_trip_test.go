package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/ibobrov/share_trip/internal/service/mocks"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTripService_CreateTrip_Allowed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	contracts := mocks.NewMockContractChecker(ctrl)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	transaction := mocks.NewMockTx(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	operationMetrics := metrics.New(prometheus.NewRegistry())
	svc := service.NewTripService(pool, useCase, operationMetrics, contracts)

	request := service.CreateTripRequest{
		ClientID: uuid.New(), CompanyID: "company-acme",
		FromPoint: "Москва", ToPoint: "Коломна",
		DepartureTime: time.Now().UTC().Add(time.Hour), AvailableSeats: 2,
	}
	domainRequest := domain.CreateTripRequest{
		ClientID: request.ClientID, CompanyID: request.CompanyID,
		FromPoint: request.FromPoint, ToPoint: request.ToPoint,
		DepartureTime: request.DepartureTime, Seats: request.AvailableSeats,
	}
	created := &domain.CreateTripResponse{
		ID: uuid.New(), ClientID: request.ClientID, CompanyID: request.CompanyID,
		FromPoint: request.FromPoint, ToPoint: request.ToPoint,
		DepartureTime: request.DepartureTime, Seats: request.AvailableSeats,
		Status: string(domain.TripStatusDraft), CreatedAt: time.Now().UTC(),
	}

	gomock.InOrder(
		contracts.EXPECT().CheckOption(ctx, request.CompanyID, "trip_creation").
			Return(service.CheckResult{Allowed: true}, nil).Times(1),
		pool.EXPECT().Begin(ctx).Return(transaction, nil).Times(1),
		useCase.EXPECT().CreateTrip(ctx, transaction, domainRequest).Return(created, nil).Times(1),
		transaction.EXPECT().Commit(ctx).Return(nil).Times(1),
		// tx always calls Rollback on exit; after COMMIT pgx returns ErrTxClosed.
		transaction.EXPECT().Rollback(ctx).Return(pgx.ErrTxClosed).Times(1),
	)

	response, err := svc.CreateTrip(ctx, request)
	require.NoError(t, err)
	require.Equal(t, service.CreateTripResponse{
		ID: created.ID, ClientID: created.ClientID, CompanyID: created.CompanyID,
		FromPoint: created.FromPoint, ToPoint: created.ToPoint,
		DepartureTime: created.DepartureTime, AvailableSeats: created.Seats,
		Status: created.Status, CreatedAt: created.CreatedAt,
	}, response)
}

func TestTripService_CreateTrip_Denied(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	contracts := mocks.NewMockContractChecker(ctrl)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)

	contracts.EXPECT().CheckOption(ctx, "company-acme", "trip_creation").
		Return(service.CheckResult{Allowed: false, Reason: "CONTRACT_EXPIRED"}, nil).Times(1)
	pool.EXPECT().Begin(gomock.Any()).Times(0)
	useCase.EXPECT().CreateTrip(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	response, err := svc.CreateTrip(ctx, service.CreateTripRequest{CompanyID: "company-acme"})
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorContains(t, err, "CONTRACT_EXPIRED")
	require.Empty(t, response)
}

func TestTripService_CreateTrip_ContractError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
	}{
		{name: "timeout", err: context.DeadlineExceeded},
		{name: "cancellation", err: context.Canceled},
		{name: "client error", err: errors.New("contract service unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			ctrl := gomock.NewController(t)
			contracts := mocks.NewMockContractChecker(ctrl)
			pool := mocks.NewMockTransactionBeginner(ctrl)
			useCase := mocks.NewMockTripUseCase(ctrl)
			svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)

			contracts.EXPECT().CheckOption(ctx, "company-acme", "trip_creation").
				Return(service.CheckResult{}, tt.err).Times(1)
			pool.EXPECT().Begin(gomock.Any()).Times(0)
			useCase.EXPECT().CreateTrip(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			response, err := svc.CreateTrip(ctx, service.CreateTripRequest{CompanyID: "company-acme"})
			require.ErrorIs(t, err, tt.err)
			require.Empty(t, response)
		})
	}
}

func TestTripService_CreateTrip_BeginFailed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	contracts := mocks.NewMockContractChecker(ctrl)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)
	beginError := errors.New("begin failed")

	gomock.InOrder(
		contracts.EXPECT().CheckOption(ctx, "company-acme", "trip_creation").
			Return(service.CheckResult{Allowed: true}, nil).Times(1),
		pool.EXPECT().Begin(ctx).Return(nil, beginError).Times(1),
	)
	useCase.EXPECT().CreateTrip(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	response, err := svc.CreateTrip(ctx, service.CreateTripRequest{CompanyID: "company-acme"})
	require.ErrorIs(t, err, beginError)
	require.Empty(t, response)
}

func TestTripService_CreateTrip_DomainError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	contracts := mocks.NewMockContractChecker(ctrl)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	transaction := mocks.NewMockTx(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)

	gomock.InOrder(
		contracts.EXPECT().CheckOption(ctx, "company-acme", "trip_creation").
			Return(service.CheckResult{Allowed: true}, nil).Times(1),
		pool.EXPECT().Begin(ctx).Return(transaction, nil).Times(1),
		useCase.EXPECT().CreateTrip(ctx, transaction, domain.CreateTripRequest{CompanyID: "company-acme"}).
			Return(nil, domain.ErrDomain).Times(1),
		transaction.EXPECT().Rollback(ctx).Return(nil).Times(1),
	)
	transaction.EXPECT().Commit(gomock.Any()).Times(0)

	response, err := svc.CreateTrip(ctx, service.CreateTripRequest{CompanyID: "company-acme"})
	require.ErrorIs(t, err, domain.ErrDomain)
	require.Empty(t, response)
}

func TestTripService_CreateTrip_CommitFailed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	contracts := mocks.NewMockContractChecker(ctrl)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	transaction := mocks.NewMockTx(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)
	commitError := errors.New("commit failed")

	gomock.InOrder(
		contracts.EXPECT().CheckOption(ctx, "company-acme", "trip_creation").
			Return(service.CheckResult{Allowed: true}, nil).Times(1),
		pool.EXPECT().Begin(ctx).Return(transaction, nil).Times(1),
		useCase.EXPECT().CreateTrip(ctx, transaction, domain.CreateTripRequest{CompanyID: "company-acme"}).
			Return(&domain.CreateTripResponse{ID: uuid.New()}, nil).Times(1),
		transaction.EXPECT().Commit(ctx).Return(commitError).Times(1),
		transaction.EXPECT().Rollback(ctx).Return(pgx.ErrTxClosed).Times(1),
	)

	response, err := svc.CreateTrip(ctx, service.CreateTripRequest{CompanyID: "company-acme"})
	require.ErrorIs(t, err, commitError)
	require.Empty(t, response)
}
