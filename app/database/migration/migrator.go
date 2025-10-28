package migration

import (
	"context"
	"errors"
	"hyperfocus/app/database"
	"log/slog"

	"github.com/elliotchance/pie/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/samber/do"
	"github.com/samber/oops"
)

type Migration interface {
	Name() string
	Version() int32
	Execute(ctx context.Context, slogger *slog.Logger, di *do.Injector, tx pgx.Tx, qtx database.TxQueries) error
}

var allMigrations = []Migration{
	&v0001InitSchema{},
}

func doExecute(
	ctx context.Context,
	slogger *slog.Logger,
	di *do.Injector,
	transactor database.TxTransactor,
	migration Migration,
) error {
	err := transactor.Transaction(ctx, func(ctx context.Context, tx pgx.Tx, qtx database.TxQueries) error {
		if err := migration.Execute(ctx, slogger, di, tx, qtx); err != nil {
			return oops.Errorf("migration.Execute: %w", err)
		}

		if err := qtx.SetSchemaVersion(ctx, migration.Version()); err != nil {
			return oops.Errorf("qtx.SetSchemaVersion: %w", err)
		}

		return nil
	})
	if err != nil {
		return oops.Errorf("transactor.Transaction: %w", err)
	}

	return nil
}

func getCurrentSchemaVersion(ctx context.Context, queries database.TxQueries) (int32, error) {
	curVersion, err := queries.GetSchemaVersion(ctx)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return 0, nil
		}

		return 0, oops.Errorf("GetSchemaVersion: %w", err)
	}

	slog.DebugContext(ctx, "Found schema version table",
		slog.Int("version", int(curVersion)),
	)

	return curVersion, nil
}

func Migrate(ctx context.Context, di *do.Injector) error {
	slog.InfoContext(ctx, "Executing migrations...")

	transactor := do.MustInvoke[database.TxTransactor](di)
	queries := do.MustInvoke[database.TxQueries](di)

	curVersion, err := getCurrentSchemaVersion(ctx, queries)
	if err != nil {
		return oops.Errorf("getCurrentSchemaVersion: %w", err)
	}

	pendingMigrations := pie.SortUsing(pie.Filter(allMigrations, func(m Migration) bool {
		return m.Version() > curVersion
	}), func(a, b Migration) bool {
		return a.Version() < b.Version()
	})

	if len(pendingMigrations) == 0 {
		slog.InfoContext(ctx, "No pending migrations")
		return nil
	}

	for _, migration := range pendingMigrations {
		slogger := slog.With(
			slog.String("name", migration.Name()),
			slog.Int("version", int(migration.Version())),
		)

		slogger.InfoContext(ctx, "Starting migration",
			slog.Int("version", int(migration.Version())),
		)

		if err = doExecute(ctx, slogger, di, transactor, migration); err != nil {
			return oops.Errorf("could not execute migration %d: %w", migration.Version(), err)
		}

		slogger.InfoContext(ctx, "Migration success",
			slog.Int("version", int(migration.Version())),
		)
	}

	log.Info("Migrations complete")

	return nil
}
