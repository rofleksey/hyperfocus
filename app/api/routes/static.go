package routes

import (
	"hyperfocus/app/api/docs"
	"hyperfocus/frontend"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

func StaticRoutes(app *fiber.App) {
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(frontend.FilesFS),
		PathPrefix: "/dist",
	}))
	app.Use("/docs", filesystem.New(filesystem.Config{
		Root: http.FS(docs.FilesFS),
	}))
}
