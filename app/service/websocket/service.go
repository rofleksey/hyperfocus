package websocket

import (
	"context"
	"fmt"
	"hyperfocus/app/api"
	"hyperfocus/app/database"
	"hyperfocus/app/dto"
	"hyperfocus/app/service/auth"
	"hyperfocus/app/service/pubsub"
	"hyperfocus/app/util/telemetry"

	"github.com/gofiber/contrib/websocket"
	"github.com/samber/do"
)

//var serviceName = "websocket"

type Service struct {
	authService   *auth.Service
	pubSubService *pubsub.Service
	tracing       *telemetry.Tracing
}

func New(di *do.Injector) (*Service, error) {
	return &Service{
		tracing:       do.MustInvoke[*telemetry.Tracing](di),
		authService:   do.MustInvoke[*auth.Service](di),
		pubSubService: do.MustInvoke[*pubsub.Service](di),
	}, nil
}

func (s *Service) NewHandler(conn *websocket.Conn, usr *database.User) *ConnectionHandler {
	ctx, cancel := context.WithCancel(context.Background())
	channels := s.getChannelsForUser(usr)

	return &ConnectionHandler{
		conn:      conn,
		channels:  channels,
		pubSub:    s.pubSubService,
		writeChan: make(chan api.IdMessage, 16),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *Service) getChannelsForUser(usr *database.User) []string {
	if usr == nil {
		return nil
	}

	channels := []string{fmt.Sprintf(dto.UserChannelFormat, usr.Username)}

	return channels
}
