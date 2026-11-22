package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InTx corre fn dentro de una transacción; si fn falla, se revierte todo.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(Querier) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
