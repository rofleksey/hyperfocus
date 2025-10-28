package migration

import (
	"context"
	"hyperfocus/app/database"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/samber/do"
	"github.com/samber/oops"
)

var _ Migration = (*v0001InitSchema)(nil)

type v0001InitSchema struct{}

func (v *v0001InitSchema) Name() string {
	return "v0001_init_schema"
}

func (v *v0001InitSchema) Version() int32 {
	return 1
}

func (v *v0001InitSchema) Execute(ctx context.Context, slogger *slog.Logger, _ *do.Injector, tx pgx.Tx, _ database.TxQueries) error {
	slogger.InfoContext(ctx, "Initializing schema...")

	_, err := tx.Exec(ctx, database.Schema)
	if err != nil {
		return oops.Errorf("failed to init database schema: %w", err)
	}

	slogger.InfoContext(ctx, "Schema successfully initialized")

	return nil
}
