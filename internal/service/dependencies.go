package service

import (
	"context"

	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/jackc/pgx/v5"
)

//go:generate $MOCKGEN -source=dependencies.go -destination=mocks/dependencies.gen.go -package=mocks
//go:generate $MOCKGEN -destination=mocks/transaction.gen.go -package=mocks github.com/jackc/pgx/v5 Tx

type TransactionBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TripUseCase interface {
	CreateTrip(ctx context.Context, tx pgx.Tx, req domain.CreateTripRequest) (*domain.CreateTripResponse, error)
	GetTrip(ctx context.Context, tx pgx.Tx, req domain.GetTripRequest) (*domain.GetTripResponse, error)
	GetTripForPublication(ctx context.Context, req domain.GetTripForPublicationRequest) (*domain.GetTripForPublicationResponse, error)
	MoveTripDraftToPublish(ctx context.Context, tx pgx.Tx, req domain.PublishTripRequest) (*domain.PublishTripResponse, error)
	SentNotificationTripPublish(ctx context.Context, tx pgx.Tx, req domain.SentNotificationTripPublishRequest) (*domain.SentNotificationTripPublishResponse, error)
}
