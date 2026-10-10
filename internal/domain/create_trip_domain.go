package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/repository"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

type CreateTripRequest struct {
	ClientID      uuid.UUID
	FromPoint     string
	ToPoint       string
	DepartureTime time.Time
	Seats         int
}

type CreateTripResponse struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	FromPoint     string
	ToPoint       string
	DepartureTime time.Time
	Seats         int
	Status        string
	CreatedAt     time.Time
}

func (u *TripUseCase) CreateTrip(
	ctx context.Context,
	tx pgx.Tx,
	req CreateTripRequest,
) (*CreateTripResponse, error) {
	logger := logctx.Logger(ctx).With(
		slog.String("layer", "usecase"),
		slog.String("usecase", "TripUsecase.CreateTrip"),
		slog.String("client_id", req.ClientID.String()),
	)
	logger.Info("create trip usecase started")

	if req.DepartureTime.Before(time.Now()) {
		return &CreateTripResponse{}, fmt.Errorf("%w: недопустимое время начало поездки", ErrDomain)
	}
	if req.Seats <= 0 {
		return &CreateTripResponse{}, fmt.Errorf("%w: недопустимое кол-во слотов в поездке", ErrDomain)
	}

	repoReq := repository.CreateTripRequest{
		ID:            uuid.New(),
		ClientID:      req.ClientID,
		FromPoint:     req.FromPoint,
		ToPoint:       req.ToPoint,
		DepartureTime: req.DepartureTime,
		Seats:         req.Seats,
		Status:        string(TripStatusDraft),
		CreatedAt:     time.Now().UTC(),
	}

	if err := u.tripRepo.CreateTrip(ctx, tx, repoReq); err != nil {
		logger.Error(
			"repository create trip failed",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("tripRepo.CreateTrip: %w", err)
	}

	if err := u.tripHistoryRepo.CreateTripHistory(ctx, tx, entity.TripHistory{
		ID:         uuid.New(),
		TripID:     repoReq.ID,
		FromStatus: nil,
		ToStatus:   repoReq.Status,
		CreatedAt:  repoReq.CreatedAt,
	}); err != nil {
		logger.Error(
			"repository create trip history failed",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("tripHistoryRepo.CreateTripHistory: %w", err)
	}

	logger.Info(
		"create trip usecase completed",
		slog.String("trip_id", repoReq.ID.String()),
	)

	return &CreateTripResponse{
		ID:            repoReq.ID,
		ClientID:      repoReq.ClientID,
		FromPoint:     repoReq.FromPoint,
		ToPoint:       repoReq.ToPoint,
		DepartureTime: repoReq.DepartureTime,
		Seats:         repoReq.Seats,
		Status:        repoReq.Status,
		CreatedAt:     repoReq.CreatedAt,
	}, nil
}
