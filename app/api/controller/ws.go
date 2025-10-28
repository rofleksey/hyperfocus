package controller

import (
	"hyperfocus/app/service/auth"
	websocket2 "hyperfocus/app/service/websocket"

	"github.com/gofiber/contrib/websocket"
	"github.com/samber/do"
)

type WS struct {
	authService      *auth.Service
	websocketService *websocket2.Service
}

func NewWS(di *do.Injector) *WS {
	return &WS{
		authService:      do.MustInvoke[*auth.Service](di),
		websocketService: do.MustInvoke[*websocket2.Service](di),
	}
}

func (c *WS) Handle(conn *websocket.Conn) {
	usr := c.authService.ExtractFromLocals(conn.Locals)
	handler := c.websocketService.NewHandler(conn, usr)
	handler.Handle()
}
