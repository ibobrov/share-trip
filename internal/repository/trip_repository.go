package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

type TripRepository struct{}

func NewTripRepository() *TripRepository {
	return &TripRepository{}
}

func (r *TripRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	trip entity.Trip,
) error {
	_, err := tx.Exec(ctx,
		`
			INSERT INTO trips (id, client_id, from_point, to_point, departure_time, seats, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
		trip.ID,
		trip.ClientID,
		trip.FromPoint,
		trip.ToPoint,
		trip.DepartureTime,
		trip.Seats,
		trip.Status,
		trip.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert trips: %w", err)
	}

	return err
}

func (r *TripRepository) GetByID(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (entity.Trip, error) {
	var trip entity.Trip

	err := tx.QueryRow(
		ctx,
		`
			SELECT id, client_id, from_point, to_point, departure_time, seats, status, created_at
			FROM trips
			WHERE id = $1
		`,
		id,
	).Scan(
		&trip.ID,
		&trip.ClientID,
		&trip.FromPoint,
		&trip.ToPoint,
		&trip.DepartureTime,
		&trip.Seats,
		&trip.Status,
		&trip.CreatedAt,
	)
	if err != nil {
		return entity.Trip{}, fmt.Errorf("get trips by id: %w", err)
	}

	return trip, nil
}
