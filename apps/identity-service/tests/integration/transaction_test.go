package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func rollbackTx(ctx context.Context, tb testing.TB, tx pgx.Tx) {
	tb.Helper()
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		tb.Errorf("rollback test transaction: %v", err)
	}
}
