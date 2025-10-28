package routes

import (
	"hyperfocus/app/api"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

func NotFoundRoute(a *fiber.App) {
	a.Use(
		func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusNotFound).JSON(api.General{
				Error:      true,
				Msg:        "route not found",
				StatusCode: http.StatusNotFound,
			})
		},
	)
}
