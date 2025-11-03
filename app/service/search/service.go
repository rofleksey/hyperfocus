package search

import (
	"context"
	"hyperfocus/app/database"
	"hyperfocus/app/util"
	"hyperfocus/app/util/telemetry"

	"github.com/samber/do"
	"github.com/samber/oops"
)

var serviceName = "search"

var maxDistance int32 = 3
var maxResults int32 = 20

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

func (s *Service) Search(ctx context.Context, query string) ([]database.Stream, error) {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "search")
	defer span.End()

	query = util.EscapeLikeQuery(query)

	data, err := s.queries.SearchStreamsByNickname(ctx, database.SearchStreamsByNicknameParams{
		Query:      query,
		Distance:   maxDistance,
		MaxResults: maxResults,
	})
	if err != nil {
		return nil, s.tracing.Error(span, oops.Errorf("SearchStreamsByNickname: %w", err)) //nolint:exhaustruct
	}

	s.tracing.Success(span)

	return data, nil
}
