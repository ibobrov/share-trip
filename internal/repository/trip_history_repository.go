package repository

import (
	"context"
	"fmt"

	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

type TripHistoryRepository struct{}

func NewTripHistoryRepository() *TripHistoryRepository {
	return &TripHistoryRepository{}
}

func (r *TripHistoryRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	trip entity.TripHistory,
) error {
	_, err := tx.Exec(ctx,
		`
			INSERT INTO trip_history (id, trip_id, from_status, to_status, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`,
		trip.ID,
		trip.TripID,
		trip.FromStatus,
		trip.ToStatus,
		trip.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert trips history: %w", err)
	}

	return err
}
