package controller

import (
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/service/limits"
	"hyperfocus/app/service/search"

	"github.com/samber/do"
)

var _ api.StrictServerInterface = (*Server)(nil)

type Server struct {
	appCtx        context.Context
	cfg           *config.Config
	dbConn        database.TxPool
	queries       database.TxQueries
	limitsService *limits.Service
	searchService *search.Service
}

func NewStrictServer(di *do.Injector) *Server {
	return &Server{
		appCtx:        do.MustInvoke[context.Context](di),
		cfg:           do.MustInvoke[*config.Config](di),
		dbConn:        do.MustInvoke[database.TxPool](di),
		queries:       do.MustInvoke[database.TxQueries](di),
		limitsService: do.MustInvoke[*limits.Service](di),
		searchService: do.MustInvoke[*search.Service](di),
	}
}
