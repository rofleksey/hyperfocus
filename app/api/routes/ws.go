package routes

import (
	"hyperfocus/app/api/controller"
	"hyperfocus/app/api/middleware"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func WSRoutes(app *fiber.App, wsController *controller.WS) {
	app.Get("/ws", middleware.WebSocketUpgrade(), websocket.New(wsController.Handle))
}
