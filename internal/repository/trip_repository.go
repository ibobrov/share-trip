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

func (r *TripRepository) CreateTrip(
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

func (r *TripRepository) UpdateTrip(
	ctx context.Context,
	tx pgx.Tx,
	trip entity.Trip,
) (ok bool, err error) {
	result, err := tx.Exec(ctx,
		`
			UPDATE trips
			SET client_id = $2, from_point = $3, to_point = $4, departure_time = $5, seats = $6, status = $7
			WHERE id = $1
		`,
		trip.ID, trip.ClientID, trip.FromPoint, trip.ToPoint, trip.DepartureTime, trip.Seats, trip.Status,
	)

	if err != nil {
		return false, fmt.Errorf("update trips: %w", err)
	}
	if result.RowsAffected() == 0 {
		return false, pgx.ErrNoRows
	}

	return true, nil
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

func (r *TripRepository) GetForUpdateByID(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (entity.Trip, error) {
	var trip entity.Trip
	err := tx.QueryRow(ctx, `
		SELECT
			id,
			client_id,
			from_point,
			to_point,
			departure_time,
			seats,
			status,
			created_at
		FROM trips
		WHERE id = $1 FOR UPDATE
	`, id).Scan(
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
		return entity.Trip{}, fmt.Errorf(
			"r.pool.QueryRow get trip by id for update: %w", err,
		)
	}
	return trip, nil
}
