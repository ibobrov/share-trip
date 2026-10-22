package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
)

type TripRepository struct {
	pool *pgxpool.Pool
}

type CreateTripRequest struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	CompanyID     string
	FromPoint     string
	ToPoint       string
	DepartureTime time.Time
	Seats         int
	Status        string
	CreatedAt     time.Time
}

func NewTripRepository(pool *pgxpool.Pool) *TripRepository {
	return &TripRepository{pool: pool}
}

type TripPublicationData struct {
	ID        uuid.UUID
	ClientID  uuid.UUID
	CompanyID string
	Status    string
}

func (r *TripRepository) GetTripForPublication(ctx context.Context, id uuid.UUID) (TripPublicationData, error) {
	var trip TripPublicationData
	err := r.pool.QueryRow(ctx, `
		SELECT id, client_id, company_id, status
		FROM trips
		WHERE id = $1
	`, id).Scan(&trip.ID, &trip.ClientID, &trip.CompanyID, &trip.Status)
	if err != nil {
		return TripPublicationData{}, fmt.Errorf("get trip for publication: %w", err)
	}
	return trip, nil
}

func (r *TripRepository) CreateTrip(
	ctx context.Context,
	tx pgx.Tx,
	req CreateTripRequest,
) error {
	logger := logctx.Logger(ctx).With(
		slog.String("layer", "repository"),
		slog.String("repository", "TripRepository"),
		slog.String("operation", "Create"),
		slog.String("trip_id", req.ID.String()),
		slog.String("client_id", req.ClientID.String()),
	)
	logger.Info("insert trip started")

	_, err := tx.Exec(ctx,
		`
			INSERT INTO trips (id, client_id, company_id, from_point, to_point, departure_time, seats, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
		req.ID,
		req.ClientID,
		req.CompanyID,
		req.FromPoint,
		req.ToPoint,
		req.DepartureTime,
		req.Seats,
		req.Status,
		req.CreatedAt)
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
	tracer := otel.Tracer("TripRepository")

	ctx, span := tracer.Start(ctx, "TripRepository.UpdateTrip")
	defer span.End()

	result, err := tx.Exec(ctx,
		`
			UPDATE trips
			SET client_id = $2, company_id = $3, from_point = $4, to_point = $5, departure_time = $6, seats = $7, status = $8
			WHERE id = $1
		`,
		trip.ID, trip.ClientID, trip.CompanyID, trip.FromPoint, trip.ToPoint, trip.DepartureTime, trip.Seats, trip.Status,
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
	tracer := otel.Tracer("TripRepository")

	ctx, span := tracer.Start(ctx, "TripRepository.GetByID")
	defer span.End()

	var trip entity.Trip

	err := tx.QueryRow(
		ctx,
		`
			SELECT id, client_id, company_id, from_point, to_point, departure_time, seats, status, created_at
			FROM trips
			WHERE id = $1
		`,
		id,
	).Scan(
		&trip.ID,
		&trip.ClientID,
		&trip.CompanyID,
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
	tracer := otel.Tracer("TripRepository")

	ctx, span := tracer.Start(ctx, "TripRepository.GetTripForUpdateByID")
	defer span.End()

	var trip entity.Trip
	err := tx.QueryRow(ctx, `
		SELECT
			id,
			client_id,
			company_id,
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
		&trip.CompanyID,
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
