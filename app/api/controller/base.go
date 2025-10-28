package controller

import (
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/service/auth"
	"hyperfocus/app/service/limits"
	"hyperfocus/app/service/settings"
	"hyperfocus/app/service/user"

	"github.com/samber/do"
)

var _ api.StrictServerInterface = (*Server)(nil)

type Server struct {
	appCtx          context.Context
	cfg             *config.Config
	dbConn          database.TxPool
	queries         database.TxQueries
	authService     *auth.Service
	limitsService   *limits.Service
	userService     *user.Service
	settingsService *settings.Service
}

func NewStrictServer(di *do.Injector) *Server {
	return &Server{
		appCtx:          do.MustInvoke[context.Context](di),
		cfg:             do.MustInvoke[*config.Config](di),
		dbConn:          do.MustInvoke[database.TxPool](di),
		queries:         do.MustInvoke[database.TxQueries](di),
		authService:     do.MustInvoke[*auth.Service](di),
		limitsService:   do.MustInvoke[*limits.Service](di),
		userService:     do.MustInvoke[*user.Service](di),
		settingsService: do.MustInvoke[*settings.Service](di),
	}
}
