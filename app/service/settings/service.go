package settings

import (
	"context"
	"database/sql"
	"errors"
	"hyperfocus/app/api"
	"hyperfocus/app/database"
	"hyperfocus/app/util/telemetry"

	"github.com/samber/do"
	"github.com/samber/oops"
)

var serviceName = "settings"

type Service struct {
	queries database.TxQueries
	tracing *telemetry.Tracing
}

func New(di *do.Injector) (*Service, error) {
	return &Service{
		queries: do.MustInvoke[database.TxQueries](di),
		tracing: do.MustInvoke[*telemetry.Tracing](di),
	}, nil
}

func (s *Service) Get(ctx context.Context) (database.Setting, error) {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "get")
	defer span.End()

	settings, err := s.queries.GetSettings(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.tracing.Success(span)
			return database.Setting{}, nil //nolint:exhaustruct
		}

		return database.Setting{}, s.tracing.Error(span, oops.Errorf("GetSettings: %w", err)) //nolint:exhaustruct
	}

	s.tracing.Success(span)

	return settings, nil
}

func (s *Service) Set(ctx context.Context, set *api.Settings) error {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "set")
	defer span.End()

	if err := s.queries.UpdateSettings(ctx, database.UpdateSettingsParams{
		ApiKey: &set.ApiKey,
	}); err != nil {
		return s.tracing.Error(span, oops.Errorf("UpdateSettings: %w", err))
	}
	s.tracing.Success(span)

	return nil
}
