package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func tx[T interface{}](
	ctx context.Context,
	pool *pgxpool.Pool,
	block func(tx pgx.Tx) (*T, error),
) (*T, error) {
	txBegin, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		err := txBegin.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Printf("rollback transaction: %v", err)
		}
	}()

	result, err := block(txBegin)
	if err != nil {
		return nil, fmt.Errorf("transaction operation: %w", err)
	}

	if err = txBegin.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}
