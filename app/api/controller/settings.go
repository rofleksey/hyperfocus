package controller

import (
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/api/mapper"

	"github.com/samber/oops"
)

func (s *Server) GetSettings(ctx context.Context, _ api.GetSettingsRequestObject) (api.GetSettingsResponseObject, error) {
	data, err := s.settingsService.Get(ctx)
	if err != nil {
		return nil, oops.Errorf("settingsService.Get: %v", err)
	}

	return api.GetSettings200JSONResponse(mapper.MapSettings(data)), nil
}

func (s *Server) SetSettings(ctx context.Context, req api.SetSettingsRequestObject) (api.SetSettingsResponseObject, error) {
	if err := s.settingsService.Set(ctx, req.Body); err != nil {
		return nil, oops.Errorf("settingsService.Set: %w", err)
	}

	return api.SetSettings200Response{}, nil
}
