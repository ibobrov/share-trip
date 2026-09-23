package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ibobrov/share_trip/internal/repository/entity"
)

type BootstrapCheckRepository struct {
	pool *pgxpool.Pool
}

func NewBootstrapCheckRepository(pool *pgxpool.Pool) *BootstrapCheckRepository {
	return &BootstrapCheckRepository{
		pool: pool,
	}
}

func (r *BootstrapCheckRepository) Create(
	ctx context.Context,
	check entity.BootstrapCheck,
) error {
	_, err := r.pool.Exec(
		ctx,
		`
			INSERT INTO bootstrap_check (id)
			VALUES ($1)
		`,
		check.ID,
	)
	if err != nil {
		return fmt.Errorf("insert bootstrap_check: %w", err)
	}

	return nil
}

func (r *BootstrapCheckRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (entity.BootstrapCheck, error) {
	var check entity.BootstrapCheck

	err := r.pool.QueryRow(
		ctx,
		`
			SELECT id, created_at
			FROM bootstrap_check
			WHERE id = $1
		`,
		id,
	).Scan(
		&check.ID,
		&check.CreatedAt,
	)
	if err != nil {
		return entity.BootstrapCheck{}, fmt.Errorf("get bootstrap_check by id: %w", err)
	}

	return check, nil
}
