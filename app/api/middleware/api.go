package middleware

import (
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/service/auth"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gofiber/fiber/v2"
	fm "github.com/oapi-codegen/fiber-middleware"
	"github.com/samber/do"
	"github.com/samber/oops"
)

func NewOpenAPIValidator(di *do.Injector) fiber.Handler {
	authService := do.MustInvoke[*auth.Service](di)

	spec, err := api.GetSwagger()
	if err != nil {
		panic(oops.Errorf("Failed to get swagger spec: %v", err))
	}

	return fm.OapiRequestValidatorWithOptions(spec,
		&fm.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
					if input.SecuritySchemeName != "Permissions" {
						return nil
					}

					if len(input.Scopes) == 0 {
						return nil
					}

					fiberCtx := fm.GetFiberContext(ctx)
					userCtx := fiberCtx.UserContext()
					curUser := authService.ExtractFromCtx(userCtx)

					for _, grantStr := range input.Scopes {
						if authService.IsGranted(curUser, api.Permission(grantStr)) {
							return nil
						}
					}

					return oops.
						Public("Access denied").
						Errorf("access denied: at least one permission is required: %v", strings.Join(input.Scopes, " | "))
				},
			},
			ErrorHandler: func(c *fiber.Ctx, message string, _ int) {
				c.Status(fiber.StatusForbidden).JSON(api.General{ //nolint:errcheck
					Error:      true,
					Msg:        message,
					StatusCode: http.StatusForbidden,
				})
			},
		})
}
