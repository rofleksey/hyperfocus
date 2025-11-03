package controller

import (
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/api/mapper"

	"github.com/elliotchance/pie/v2"
	"github.com/samber/oops"
)

func (s *Server) SearchPlayers(ctx context.Context, request api.SearchPlayersRequestObject) (api.SearchPlayersResponseObject, error) {
	data, err := s.searchService.Search(ctx, request.Body.Query)
	if err != nil {
		return nil, oops.Errorf("searchService.Search: %w", err)
	}

	return api.SearchPlayers200JSONResponse{
		Data: pie.Map(data, mapper.MapStream),
	}, nil
}
