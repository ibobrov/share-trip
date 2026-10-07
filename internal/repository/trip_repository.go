package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
)

type TripRepository struct {
	metrics *metrics.Metrics
}

func NewTripRepository(metrics *metrics.Metrics) *TripRepository {
	return &TripRepository{
		metrics: metrics,
	}
}

func (r *TripRepository) CreateTrip(
	ctx context.Context,
	tx pgx.Tx,
	trip entity.Trip,
) error {
	started := time.Now()
	metricRsl := "success"

	defer func() {
		r.metrics.RepositoryQueryTotal.WithLabelValues(
			"trip_create",
			metricRsl,
		).Inc()

		r.metrics.RepositoryQueryDuration.WithLabelValues(
			"trip_create",
			metricRsl,
		).Observe(time.Since(started).Seconds())
	}()

	logger := logctx.Logger(ctx).With(
		slog.String("layer", "repository"),
		slog.String("repository", "TripRepository"),
		slog.String("operation", "Create"),
		slog.String("trip_id", trip.ID.String()),
		slog.String("client_id", trip.ClientID.String()),
	)
	logger.Info("insert trip started")

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
		logger.Error(
			"insert trip failed",
			slog.Any("error", err),
		)
		return fmt.Errorf("insert trips: %w", err)
	}

	logger.Info("insert trip completed")
	return err
}

func (r *TripRepository) UpdateTrip(
	ctx context.Context,
	tx pgx.Tx,
	trip entity.Trip,
) (ok bool, err error) {
	started := time.Now()
	metricRsl := "success"

	defer func() {
		r.metrics.RepositoryQueryTotal.WithLabelValues(
			"update_create",
			metricRsl,
		).Inc()

		r.metrics.RepositoryQueryDuration.WithLabelValues(
			"update_create",
			metricRsl,
		).Observe(time.Since(started).Seconds())
	}()

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

func (r *TripRepository) GetTripByID(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (entity.Trip, error) {
	started := time.Now()
	metricRsl := "success"

	defer func() {
		r.metrics.RepositoryQueryTotal.WithLabelValues(
			"get_trip_by_id",
			metricRsl,
		).Inc()

		r.metrics.RepositoryQueryDuration.WithLabelValues(
			"get_trip_by_id",
			metricRsl,
		).Observe(time.Since(started).Seconds())
	}()

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

func (r *TripRepository) GetTripForUpdateByID(
	ctx context.Context,
	tx pgx.Tx,
	id uuid.UUID,
) (entity.Trip, error) {
	started := time.Now()
	metricRsl := "success"

	defer func() {
		r.metrics.RepositoryQueryTotal.WithLabelValues(
			"get_trip_for_update_by_id",
			metricRsl,
		).Inc()

		r.metrics.RepositoryQueryDuration.WithLabelValues(
			"get_trip_for_update_by_id",
			metricRsl,
		).Observe(time.Since(started).Seconds())
	}()

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
