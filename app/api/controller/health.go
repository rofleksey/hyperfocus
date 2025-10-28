package controller

import (
	"context"
	"hyperfocus/app/api"

	"go.szostok.io/version"
)

func (s *Server) HealthCheck(_ context.Context, _ api.HealthCheckRequestObject) (api.HealthCheckResponseObject, error) {
	info := version.Get()

	return api.HealthCheck200JSONResponse{
		Version:   info.Version,
		BuildDate: info.BuildDate,
	}, nil
}
