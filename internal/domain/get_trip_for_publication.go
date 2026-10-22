package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GetTripForPublicationRequest struct {
	TripID uuid.UUID
}

type GetTripForPublicationResponse struct {
	ID        uuid.UUID
	ClientID  uuid.UUID
	CompanyID string
	Status    string
}

func (u *TripUseCase) GetTripForPublication(ctx context.Context, req GetTripForPublicationRequest) (*GetTripForPublicationResponse, error) {
	trip, err := u.tripRepo.GetTripForPublication(ctx, req.TripID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: trip %s", ErrNotFound, req.TripID)
	}
	if err != nil {
		return nil, fmt.Errorf("tripRepository.GetTripForPublication: %w", err)
	}
	return &GetTripForPublicationResponse{
		ID: trip.ID, ClientID: trip.ClientID, CompanyID: trip.CompanyID, Status: trip.Status,
	}, nil
}
