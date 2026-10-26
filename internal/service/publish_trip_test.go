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

func TestTripService_PublishTrip_ContractDenied(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	contracts := mocks.NewMockContractChecker(ctrl)
	svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)
	trip := &domain.GetTripForPublicationResponse{
		ID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-from-trip", Status: "draft",
	}
	readFinished := useCase.EXPECT().GetTripForPublication(gomock.Any(), domain.GetTripForPublicationRequest{TripID: trip.ID}).Return(trip, nil).Times(1)
	contracts.EXPECT().CheckOption(gomock.Any(), trip.CompanyID, "trip_publication").
		Return(service.CheckResult{Reason: "CONTRACT_EXPIRED"}, nil).After(readFinished).Times(1)
	pool.EXPECT().Begin(gomock.Any()).Times(0)
	useCase.EXPECT().MoveTripDraftToPublish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	useCase.EXPECT().SentNotificationTripPublish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	response, err := svc.PublishTrip(t.Context(), service.PublishTripRequest{TripID: trip.ID, ClientID: trip.ClientID})
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorContains(t, err, "CONTRACT_EXPIRED")
	require.Empty(t, response)
}

func TestTripService_PublishTrip_ReadError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	contracts := mocks.NewMockContractChecker(ctrl)
	svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)
	tripID := uuid.New()
	useCase.EXPECT().GetTripForPublication(gomock.Any(), domain.GetTripForPublicationRequest{TripID: tripID}).
		Return(nil, domain.ErrNotFound).Times(1)
	pool.EXPECT().Begin(gomock.Any()).Times(0)
	contracts.EXPECT().CheckOption(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	useCase.EXPECT().MoveTripDraftToPublish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.PublishTrip(t.Context(), service.PublishTripRequest{TripID: tripID, ClientID: uuid.New()})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTripService_PublishTrip_ContractTimeout(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	pool := mocks.NewMockTransactionBeginner(ctrl)
	useCase := mocks.NewMockTripUseCase(ctrl)
	contracts := mocks.NewMockContractChecker(ctrl)
	svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)
	trip := &domain.GetTripForPublicationResponse{
		ID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-from-trip", Status: "draft",
	}
	readFinished := useCase.EXPECT().GetTripForPublication(gomock.Any(), domain.GetTripForPublicationRequest{TripID: trip.ID}).Return(trip, nil).Times(1)
	contracts.EXPECT().CheckOption(gomock.Any(), trip.CompanyID, "trip_publication").
		Return(service.CheckResult{}, context.DeadlineExceeded).After(readFinished).Times(1)
	pool.EXPECT().Begin(gomock.Any()).Times(0)
	useCase.EXPECT().MoveTripDraftToPublish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	useCase.EXPECT().SentNotificationTripPublish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, err := svc.PublishTrip(t.Context(), service.PublishTripRequest{TripID: trip.ID, ClientID: trip.ClientID})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTripService_PublishTrip_Preconditions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		status      string
		anotherUser bool
		wantError   error
	}{
		{name: "another driver", status: "draft", anotherUser: true, wantError: domain.ErrForbidden},
		{name: "already published", status: "published", wantError: domain.ErrTripIsAlreadyPublished},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			pool := mocks.NewMockTransactionBeginner(ctrl)
			useCase := mocks.NewMockTripUseCase(ctrl)
			contracts := mocks.NewMockContractChecker(ctrl)
			svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)
			trip := &domain.GetTripForPublicationResponse{
				ID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-from-trip", Status: tt.status,
			}
			useCase.EXPECT().GetTripForPublication(gomock.Any(), domain.GetTripForPublicationRequest{TripID: trip.ID}).Return(trip, nil).Times(1)
			contracts.EXPECT().CheckOption(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			pool.EXPECT().Begin(gomock.Any()).Times(0)
			useCase.EXPECT().MoveTripDraftToPublish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			clientID := trip.ClientID
			if tt.anotherUser {
				clientID = uuid.New()
			}

			_, err := svc.PublishTrip(t.Context(), service.PublishTripRequest{TripID: trip.ID, ClientID: clientID})
			require.ErrorIs(t, err, tt.wantError)
		})
	}
}

func TestTripService_PublishTrip_AfterContractAllowed(t *testing.T) {
	t.Parallel()
	commitError := errors.New("commit failed")
	outboxError := errors.New("outbox insert failed")
	tests := []struct {
		name        string
		domainError error
		outboxError error
		commitError error
		wantError   error
	}{
		{name: "trip and outbox committed together"},
		{name: "domain rejects changed state", domainError: domain.ErrConflict, wantError: domain.ErrConflict},
		{name: "outbox failure prevents event", outboxError: outboxError, wantError: outboxError},
		{name: "commit failure prevents event", commitError: commitError, wantError: commitError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			pool := mocks.NewMockTransactionBeginner(ctrl)
			useCase := mocks.NewMockTripUseCase(ctrl)
			contracts := mocks.NewMockContractChecker(ctrl)
			writeTx := mocks.NewMockTx(ctrl)
			svc := service.NewTripService(pool, useCase, metrics.New(prometheus.NewRegistry()), contracts)
			trip := &domain.GetTripForPublicationResponse{
				ID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-from-trip", Status: "draft",
			}
			occurredAt := time.Now().UTC()
			eventID := uuid.New()
			read := useCase.EXPECT().GetTripForPublication(gomock.Any(), domain.GetTripForPublicationRequest{TripID: trip.ID}).Return(trip, nil).Times(1)
			check := contracts.EXPECT().CheckOption(gomock.Any(), trip.CompanyID, "trip_publication").
				Return(service.CheckResult{Allowed: true}, nil).After(read).Times(1)
			begin := pool.EXPECT().Begin(gomock.Any()).Return(writeTx, nil).After(check).Times(1)
			publish := useCase.EXPECT().MoveTripDraftToPublish(gomock.Any(), writeTx, domain.PublishTripRequest{
				TripID: trip.ID, ClientID: trip.ClientID,
			}).Return(&domain.PublishTripResponse{
				TripID: trip.ID, ClientID: trip.ClientID, CompanyID: trip.CompanyID, PublishedAt: occurredAt,
			}, tt.domainError).After(begin).Times(1)
			last := publish
			if tt.domainError == nil {
				outbox := useCase.EXPECT().SentNotificationTripPublish(gomock.Any(), writeTx, domain.SentNotificationTripPublishRequest{
					TripID: trip.ID, ClientID: trip.ClientID, CompanyID: trip.CompanyID, OccurredAt: occurredAt,
				}).Return(&domain.SentNotificationTripPublishResponse{EventID: eventID}, tt.outboxError).After(publish).Times(1)
				last = outbox
			}
			if tt.domainError == nil && tt.outboxError == nil {
				commit := writeTx.EXPECT().Commit(gomock.Any()).Return(tt.commitError).After(last).Times(1)
				writeTx.EXPECT().Rollback(gomock.Any()).Return(pgx.ErrTxClosed).After(commit).Times(1)
			} else {
				writeTx.EXPECT().Rollback(gomock.Any()).Return(nil).After(last).Times(1)
				writeTx.EXPECT().Commit(gomock.Any()).Times(0)
			}
			response, err := svc.PublishTrip(t.Context(), service.PublishTripRequest{TripID: trip.ID, ClientID: trip.ClientID})
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
				require.Empty(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, trip.ID, response.TripID)
			require.Equal(t, eventID, response.EventID)
		})
	}
}
