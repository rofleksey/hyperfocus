package database

import (
	"context"
	"hyperfocus/app/util/telemetry"

	"github.com/jackc/pgx/v5"
	"github.com/samber/oops"
)

type Transactor struct {
	dbConn  TxPool
	queries TxQueries
	tracing *telemetry.Tracing
}

func NewTransactor(dbConn TxPool, queries TxQueries, tracing *telemetry.Tracing) *Transactor {
	return &Transactor{
		dbConn:  dbConn,
		queries: queries,
		tracing: tracing,
	}
}

func (t *Transactor) Transaction(ctx context.Context, callback func(ctx context.Context, tx pgx.Tx, qtx TxQueries) error) error {
	ctx, span := t.tracing.StartSpan(ctx, "transaction")
	defer span.End()

	tx, err := t.dbConn.Begin(ctx)
	if err != nil {
		return t.tracing.Error(span, oops.Errorf("Begin: %w", err))
	}
	defer tx.Rollback(ctx)

	qtx := t.queries.WithTx(tx)

	if err = callback(ctx, tx, qtx); err != nil {
		return t.tracing.Error(span, oops.Errorf("callback: %w", err))
	}

	if err = tx.Commit(ctx); err != nil {
		return t.tracing.Error(span, oops.Errorf("Commit: %w", err))
	}

	t.tracing.Success(span)

	return nil
}
