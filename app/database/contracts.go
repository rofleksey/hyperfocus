package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type TxPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type TxQueries interface {
	Querier
	WithTx(tx pgx.Tx) *Queries
}

type TxTransactor interface {
	Transaction(ctx context.Context, callback func(ctx context.Context, tx pgx.Tx, qtx TxQueries) error) error
}
