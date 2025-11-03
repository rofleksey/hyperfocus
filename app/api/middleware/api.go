package middleware

import (
	"hyperfocus/app/api"
	"net/http"

	"github.com/gofiber/fiber/v2"
	fm "github.com/oapi-codegen/fiber-middleware"
	"github.com/samber/do"
	"github.com/samber/oops"
)

func NewOpenAPIValidator(di *do.Injector) fiber.Handler {
	spec, err := api.GetSwagger()
	if err != nil {
		panic(oops.Errorf("Failed to get swagger spec: %w", err))
	}

	return fm.OapiRequestValidatorWithOptions(spec,
		&fm.Options{
			ErrorHandler: func(c *fiber.Ctx, message string, _ int) {
				c.Status(fiber.StatusForbidden).JSON(api.General{ //nolint:errcheck
					Error:      true,
					Msg:        message,
					StatusCode: http.StatusForbidden,
				})
			},
		})
}
